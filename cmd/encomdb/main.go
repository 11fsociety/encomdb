package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/11fsociety/encomdb/internal/dbs"
	"github.com/11fsociety/encomdb/internal/tunnel"
	"github.com/11fsociety/encomdb/internal/ui"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

const version = "0.1.0"

func main() {
	app := pocketbase.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var tun *tunnel.Tunnel

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := dbs.EnsureCollections(app); err != nil {
			return err
		}
		if err := dbs.EnsureDefaultAdmin(app); err != nil {
			log.Printf("warning: could not seed default admin: %v", err)
		}

		root := filepath.Join(app.DataDir(), "encom_dbs")
		if err := os.MkdirAll(root, 0o755); err != nil {
			return err
		}
		mgr, err := dbs.NewManager(app, root)
		if err != nil {
			return err
		}

		publicHost := os.Getenv("ENCOMDB_PUBLIC_HOST")
		if publicHost == "" {
			publicHost = "http://" + e.Server.Addr
		}
		mgr.SetPublicHost(publicHost)

		encomGroup := e.Router.Group("/api/encom")
		dbs.RegisterRoutes(encomGroup, mgr, app)

		e.Router.GET("/dashboard", func(re *core.RequestEvent) error {
			return re.HTML(http.StatusOK, ui.DashboardHTML)
		})
		e.Router.GET("/dashboard/{path...}", func(re *core.RequestEvent) error {
			return re.HTML(http.StatusOK, ui.DashboardHTML)
		})

		// Start the Cloudflare tunnel supervisor if enabled and cloudflared is present.
		if tunnel.Enabled() {
			bin := tunnel.Locate()
			if bin == "" {
				log.Printf("[tunnel] cloudflared not found in PATH or $HOME/bin — running LAN-only. See README for install steps.")
			} else {
				// cloudflared connects out to a loopback target — 0.0.0.0 is not
				// a valid dial address for the child. Convert to 127.0.0.1.
				target := e.Server.Addr
				if idx := strings.LastIndex(target, ":"); idx >= 0 {
					target = "127.0.0.1" + target[idx:]
				} else {
					target = "127.0.0.1:" + target
				}
				tun = tunnel.New(target, bin)
				tun.OnURL(func(u string) {
					mgr.SetTunnelURL(u)
				})
				tun.Start(ctx)
			}
		} else {
			log.Printf("[tunnel] disabled via ENCOMDB_TUNNEL=0")
		}

		log.Printf("encomdb %s ready — admin: /_/  dashboard: /dashboard", version)
		return e.Next()
	})

	// Graceful shutdown for the tunnel.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		if tun != nil {
			log.Printf("[tunnel] stopping…")
			tun.Stop()
		}
		cancel()
	}()

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
