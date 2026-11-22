/**
##
## OverDrive 2026
## All Technical rights reserved
##
## main.go - Gateway application entrypoint and HTTP server startup.
##
*/

// Package main boots the API gateway process and HTTP server.

package main

import (
	"log"
	"overdrive/gateway/internal/app"
	"overdrive/gateway/internal/config"
	"overdrive/shared/bootstrap"
)

// main serves without argument; `healthcheck` probes the running service (see bootstrap.Main).
// It exits non-zero on failure.
func main() {
	bootstrap.Main("gateway", run)
}

// run builds the gateway and serves until SIGINT/SIGTERM. Returning instead of calling log.Fatal
// lets the deferred runtime cleanup run on every exit path.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	shutdownTimeout, err := bootstrap.ShutdownTimeout()
	if err != nil {
		return err
	}

	runtime, err := app.Build(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := runtime.Close(); closeErr != nil {
			log.Printf("unable to close runtime dependencies: %v", closeErr)
		}
	}()

	server := bootstrap.NewServer(":"+cfg.HTTPPort, runtime.Handler, bootstrap.DefaultServerLimits())

	log.Printf("gateway listening on :%s", cfg.HTTPPort)

	if err := bootstrap.Serve(server, shutdownTimeout); err != nil {
		return err
	}
	log.Printf("gateway stopped")
	return nil
}
