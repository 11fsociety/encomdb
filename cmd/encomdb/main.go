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

	"github.com/11fsociety/encomdb/internal/rocketdb"
	"github.com/11fsociety/encomdb/internal/tunnel"
	"github.com/11fsociety/encomdb/internal/ui"
	pb "github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

const version = "0.1.0"

func main() {
	app := pb.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var tun *tunnel.Tunnel

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := rocketdb.EnsureCollections(app); err != nil {
			return err
		}
		if err := rocketdb.EnsureDefaultAdmin(app); err != nil {
			log.Printf("warning: could not seed default admin: %v", err)
		}

		root := filepath.Join(app.DataDir(), "rocketdb")
		if err := os.MkdirAll(root, 0o755); err != nil {
			return err
		}
		mgr, err := rocketdb.NewManager(app, root)
		if err != nil {
			return err
		}

		publicHost := os.Getenv("ENCOMDB_PUBLIC_HOST")
		if publicHost == "" {
			publicHost = "http://" + e.Server.Addr
		}
		mgr.SetPublicHost(publicHost)

		rocketGroup := e.Router.Group("/api/rocketdb")
		rocketdb.RegisterRoutes(rocketGroup, mgr, app)

		e.Router.GET("/dashboard", func(re *core.RequestEvent) error {
			return re.HTML(http.StatusOK, ui.DashboardHTML)
		})
		e.Router.GET("/dashboard/{path...}", func(re *core.RequestEvent) error {
			return re.HTML(http.StatusOK, ui.DashboardHTML)
		})

		// Start the serveo.net tunnel supervisor if enabled and ssh is present.
		if tunnel.Enabled() {
			sshBin := tunnel.LocateSSH()
			if sshBin == "" {
				log.Printf("[tunnel] ssh not found — running LAN-only. In Termux: pkg install openssh")
			} else {
				target := e.Server.Addr
				if idx := strings.LastIndex(target, ":"); idx >= 0 {
					target = "127.0.0.1" + target[idx:]
				} else {
					target = "127.0.0.1:" + target
				}
				tun = tunnel.New(target, sshBin, tunnel.Subdomain())
				poster := tunnel.NewRegistryPoster()
				if poster.Enabled() {
					log.Printf("[tunnel/registry] will publish URL to EncomPortal registry")
				}
				tun.OnURL(func(u string) {
					mgr.SetTunnelURL(u)
					go poster.Post(context.Background(), u)
				})
				tun.Start(ctx)
			}
		} else {
			log.Printf("[tunnel] disabled via ENCOMDB_TUNNEL=0")
		}

		log.Printf("EncomPortal core %s ready — admin: /_/  dashboard: /dashboard", version)
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
