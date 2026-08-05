package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// RegistryPoster forwards the current tunnel URL to a remote HTTP registry
// (e.g. the EncomPortal Vercel deployment). Enabled when
// ENCOMDB_TUNNEL_REGISTRY_URL is set; token is optional.
//
// Env:
//
//	ENCOMDB_TUNNEL_REGISTRY_URL   e.g. https://encomportal.vercel.app/api/tunnel
//	ENCOMDB_TUNNEL_REGISTRY_TOKEN optional Bearer secret
//
// Post() sends the URL once. StartKeepAlive() runs a background goroutine
// that re-posts the latest known URL every 60s so the portal's in-memory
// store recovers from Vercel serverless cold starts within one minute.
type RegistryPoster struct {
	url    string
	token  string
	client *http.Client

	mu      sync.RWMutex
	lastURL string
}

func NewRegistryPoster() *RegistryPoster {
	return &RegistryPoster{
		url:    os.Getenv("ENCOMDB_TUNNEL_REGISTRY_URL"),
		token:  os.Getenv("ENCOMDB_TUNNEL_REGISTRY_TOKEN"),
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled reports whether the registry URL is configured.
func (r *RegistryPoster) Enabled() bool {
	return r.url != ""
}

// Post sends the current tunnel URL to the registry.
func (r *RegistryPoster) Post(ctx context.Context, tunnelURL string) {
	if !r.Enabled() {
		return
	}
	r.mu.Lock()
	r.lastURL = tunnelURL
	r.mu.Unlock()
	r.post(ctx, tunnelURL)
}

// StartKeepAlive re-posts the latest known URL every 60s. Returns when ctx cancels.
func (r *RegistryPoster) StartKeepAlive(ctx context.Context) {
	if !r.Enabled() {
		return
	}
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.RLock()
			u := r.lastURL
			r.mu.RUnlock()
			if u == "" {
				continue
			}
			r.post(ctx, u)
		}
	}
}

func (r *RegistryPoster) post(ctx context.Context, tunnelURL string) {
	body, _ := json.Marshal(map[string]string{"url": tunnelURL})
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
		if err != nil {
			log.Printf("[tunnel/registry] new request error: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if r.token != "" {
			req.Header.Set("Authorization", "Bearer "+r.token)
		}
		resp, err := r.client.Do(req)
		if err != nil {
			log.Printf("[tunnel/registry] POST error (attempt %d): %v", attempt+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[tunnel/registry] registered %s at %s", tunnelURL, r.url)
			return
		}
		log.Printf("[tunnel/registry] non-2xx from %s: %d", r.url, resp.StatusCode)
		return
	}
}
