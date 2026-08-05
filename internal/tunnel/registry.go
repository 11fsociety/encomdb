package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// RegistryPoster forwards the current tunnel URL to a remote HTTP registry
// (e.g. the EncomPortal Vercel deployment). If neither URL nor TOKEN is set
// via env, this is a no-op.
//
// Env:
//
//	ENCOMDB_TUNNEL_REGISTRY_URL   e.g. https://encomportal.vercel.app/api/tunnel
//	ENCOMDB_TUNNEL_REGISTRY_TOKEN shared secret (Bearer)
type RegistryPoster struct {
	url    string
	token  string
	client *http.Client
}

func NewRegistryPoster() *RegistryPoster {
	return &RegistryPoster{
		url:    os.Getenv("ENCOMDB_TUNNEL_REGISTRY_URL"),
		token:  os.Getenv("ENCOMDB_TUNNEL_REGISTRY_TOKEN"),
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled reports whether both env vars are set.
func (r *RegistryPoster) Enabled() bool {
	return r.url != "" && r.token != ""
}

// Post sends the current tunnel URL to the registry. Retries once on
// transient failures. Logs and swallows errors — the registry is a
// convenience layer, not a correctness dependency.
func (r *RegistryPoster) Post(ctx context.Context, tunnelURL string) {
	if !r.Enabled() {
		return
	}
	body, _ := json.Marshal(map[string]string{"url": tunnelURL})
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
		if err != nil {
			log.Printf("[tunnel/registry] new request error: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+r.token)
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
