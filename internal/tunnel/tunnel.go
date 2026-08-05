// Package tunnel runs a Cloudflare quick tunnel as a supervised child process.
//
// It looks for the `cloudflared` binary in PATH or a couple of well-known
// Termux locations, then spawns `cloudflared tunnel --url http://<addr>`.
// It scrapes the tunnel URL from cloudflared's stderr and exposes it via the
// URL() method. If cloudflared crashes it restarts with exponential backoff.
//
// Set ENCOMDB_TUNNEL=0 to disable.
package tunnel

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

type Tunnel struct {
	targetAddr string // e.g. "localhost:8090"
	binary     string // path to cloudflared

	mu  sync.RWMutex
	url string

	cancel context.CancelFunc
	done   chan struct{}

	onURL func(string)
}

// urlRegex captures the printed "https://xxx.trycloudflare.com" line.
var urlRegex = regexp.MustCompile(`https://[a-z0-9\-]+\.trycloudflare\.com`)

// Locate finds a usable cloudflared binary. Returns "" if none found.
func Locate() string {
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}
	candidates := []string{}
	if h, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(h, "bin", "cloudflared"),
			filepath.Join(h, ".local", "bin", "cloudflared"),
		)
	}
	// Termux default prefix
	candidates = append(candidates,
		"/data/data/com.termux/files/home/bin/cloudflared",
		"/data/data/com.termux/files/usr/bin/cloudflared",
	)
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// New constructs a tunnel controller. It does not start anything.
func New(targetAddr, binary string) *Tunnel {
	return &Tunnel{
		targetAddr: targetAddr,
		binary:     binary,
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

// Start begins the supervisor goroutine. Blocks until ctx is cancelled.
func (t *Tunnel) Start(ctx context.Context) {
	if t.binary == "" {
		log.Printf("[tunnel] cloudflared binary not found; skipping tunnel setup")
		return
	}
	log.Printf("[tunnel] using %s -> http://%s", t.binary, t.targetAddr)

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	go t.supervise(ctx)
}

// Stop terminates the supervisor and its child.
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
			log.Printf("[tunnel] cloudflared exited: %v (ran for %s)", err, time.Since(start).Round(time.Second))
		} else {
			log.Printf("[tunnel] cloudflared exited cleanly (ran for %s)", time.Since(start).Round(time.Second))
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
	cmd := exec.CommandContext(ctx, t.binary, "tunnel", "--no-autoupdate", "--url", "http://"+t.targetAddr)
	cmd.Env = append(os.Environ(), "TUNNEL_LOGGER_LEVEL=info")
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
	go t.scanForURL(stderr)
	go t.scanForURL(stdout)
	return cmd.Wait()
}

func (t *Tunnel) scanForURL(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 512*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if u := urlRegex.FindString(line); u != "" {
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

// Enabled reports whether the ENCOMDB_TUNNEL env var says the tunnel should run.
// Default: enabled. Only "0" or "false" (case-insensitive) disables.
func Enabled() bool {
	v := os.Getenv("ENCOMDB_TUNNEL")
	switch v {
	case "0", "false", "FALSE", "False", "no", "off":
		return false
	}
	return true
}

var _ = errors.New // reserved
