/**
##
## OverDrive 2026
## All Technical rights reserved
##
## main.go - Service entrypoint and HTTP server bootstrap for user-data-service.
##
*/

package main

import (
	"fmt"
	"log"
	"os"
	db "overdrive/services/user-data-service/resources/db"
	httpadapter "overdrive/services/user-data-service/src/adapters/http"
	prismaadapter "overdrive/services/user-data-service/src/adapters/repository/prisma"
	"overdrive/services/user-data-service/src/core/usecases"
	"overdrive/shared/authjwt"
	"overdrive/shared/bootstrap"
)

// main serves without argument; `healthcheck` probes the running service (see bootstrap.Main).
// It exits non-zero on failure.
func main() {
	bootstrap.Main("user-data-service", run)
}

// run wires the dependencies and serves until SIGINT/SIGTERM. Returning instead of calling
// log.Fatal lets the deferred cleanup (database disconnect) run on every exit path.
func run() error {
	cfg, err := bootstrap.LoadConfig(
		"user-data-service",
		"User profile, preferences, and saved data service.",
	)
	if err != nil {
		return err
	}
	shutdownTimeout, err := bootstrap.ShutdownTimeout()
	if err != nil {
		return err
	}

	// AUTH_JWT_SECRET verifies the users' access tokens (issued by auth-service). Without it no
	// preset request can be authenticated, so the service refuses to start.
	verifier, err := authjwt.NewVerifier(os.Getenv("AUTH_JWT_SECRET"))
	if err != nil {
		return fmt.Errorf("AUTH_JWT_SECRET: %w (generate one with: openssl rand -hex 32)", err)
	}

	client := db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
		return err
	}
	defer func() {
		if err := client.Prisma.Disconnect(); err != nil {
			log.Printf("failed to disconnect prisma client: %v", err)
		}
	}()

	usecase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	controller := httpadapter.NewHealthController(usecase)

	presetRepository := prismaadapter.NewPresetRepository(client)
	presetUsecase := usecases.NewPresetUseCase(presetRepository)
	presetController := httpadapter.NewPresetController(presetUsecase)

	server := bootstrap.NewServer(":"+cfg.HTTPPort, httpadapter.NewRouter(controller, presetController, verifier), bootstrap.DefaultServerLimits())

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := bootstrap.Serve(server, shutdownTimeout); err != nil {
		return err
	}
	log.Printf("%s stopped", cfg.ServiceName)
	return nil
}
