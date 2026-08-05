// Package tunnel exposes the local EncomDB server to the public internet.
//
// Default provider is serveo.net (SSH-based reverse tunnel, no account, no
// binary install — just uses the ssh client Termux already ships with).
// Set ENCOMDB_TUNNEL=0 to disable. Set ENCOMDB_TUNNEL_SUBDOMAIN=<name> to
// request a specific subdomain like https://<name>.serveo.net (falls back
// to a random one if that name is taken).
//
// Restart backoff: 3s -> 6s -> ... -> 60s cap. Resets to 3s after any run
// that stayed alive for at least 30s.
package tunnel

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

type Tunnel struct {
	targetAddr string // e.g. "127.0.0.1:8090"
	sshBinary  string // path to ssh
	subdomain  string // empty = random

	mu  sync.RWMutex
	url string

	cancel context.CancelFunc
	done   chan struct{}

	onURL func(string)
}

// serveoURLRegex catches the URL line: "Forwarding HTTP traffic from https://xxx.serveo.net"
var serveoURLRegex = regexp.MustCompile(`https://[a-zA-Z0-9\-]+\.serveo\.net`)

// LocateSSH finds an ssh client. Empty return = not available.
func LocateSSH() string {
	if p, err := exec.LookPath("ssh"); err == nil {
		return p
	}
	// Termux common locations
	candidates := []string{
		"/data/data/com.termux/files/usr/bin/ssh",
		"/usr/bin/ssh",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// New constructs a tunnel. Does not start anything.
func New(targetAddr, sshBinary, subdomain string) *Tunnel {
	return &Tunnel{
		targetAddr: targetAddr,
		sshBinary:  sshBinary,
		subdomain:  subdomain,
		done:       make(chan struct{}),
	}
}

// OnURL registers a callback invoked whenever a new tunnel URL is parsed.
func (t *Tunnel) OnURL(fn func(string)) { t.onURL = fn }

// URL returns the current tunnel URL, or "" if not yet ready.
func (t *Tunnel) URL() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.url
}

// Start begins the supervisor goroutine.
func (t *Tunnel) Start(ctx context.Context) {
	if t.sshBinary == "" {
		log.Printf("[tunnel] ssh not found; skipping tunnel setup")
		return
	}
	log.Printf("[tunnel] using serveo.net via %s -> http://%s (subdomain=%q)",
		t.sshBinary, t.targetAddr, t.subdomain)

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	go t.supervise(ctx)
}

func (t *Tunnel) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
	select {
	case <-t.done:
	case <-time.After(5 * time.Second):
	}
}

func (t *Tunnel) supervise(ctx context.Context) {
	defer close(t.done)
	backoff := 3 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}
		start := time.Now()
		err := t.runOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("[tunnel] ssh exited: %v (ran for %s)", err, time.Since(start).Round(time.Second))
		} else {
			log.Printf("[tunnel] ssh exited cleanly (ran for %s)", time.Since(start).Round(time.Second))
		}
		if time.Since(start) > 30*time.Second {
			backoff = 3 * time.Second
		}
		log.Printf("[tunnel] restarting in %s…", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (t *Tunnel) runOnce(ctx context.Context) error {
	// If a subdomain was requested, serveo maps it via the -R remote binding.
	// Format: -R <subdomain>:80:<host>:<port>
	//   Random subdomain: -R 80:127.0.0.1:8090
	//   Fixed subdomain:  -R foo:80:127.0.0.1:8090
	remote := "80:" + t.targetAddr
	if t.subdomain != "" {
		remote = t.subdomain + ":" + remote
	}

	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "ExitOnForwardFailure=yes",
		"-T", // no tty
		"-R", remote,
		"serveo.net",
	}
	log.Printf("[tunnel] spawning: ssh %s", quoteArgs(args))

	cmd := exec.CommandContext(ctx, t.sshBinary, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	drained := make(chan struct{}, 2)
	go func() { t.scanForURL(stderr); drained <- struct{}{} }()
	go func() { t.scanForURL(stdout); drained <- struct{}{} }()
	err = cmd.Wait()
	<-drained
	<-drained
	return err
}

func (t *Tunnel) scanForURL(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 512*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if u := serveoURLRegex.FindString(line); u != "" {
			t.set(u)
		}
		if line != "" {
			log.Printf("[tunnel] %s", line)
		}
	}
	_ = scanner.Err()
}

func (t *Tunnel) set(u string) {
	t.mu.Lock()
	changed := t.url != u
	t.url = u
	t.mu.Unlock()
	if changed {
		log.Printf("[tunnel] PUBLIC URL: %s", u)
		if t.onURL != nil {
			t.onURL(u)
		}
	}
}

// Enabled reports whether ENCOMDB_TUNNEL says we should run the tunnel.
// Default: enabled. "0", "false", "no", "off" disable.
func Enabled() bool {
	v := os.Getenv("ENCOMDB_TUNNEL")
	switch v {
	case "0", "false", "FALSE", "False", "no", "off":
		return false
	}
	return true
}

// Subdomain returns the requested serveo subdomain (may be empty for random).
func Subdomain() string {
	return os.Getenv("ENCOMDB_TUNNEL_SUBDOMAIN")
}

func quoteArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
