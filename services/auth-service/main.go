/**
##
## OverDrive 2026
## All Technical rights reserved
##
## main.go - Service entrypoint and HTTP server bootstrap for auth-service.
##
*/

package main

import (
	"fmt"
	"log"
	"os"
	db "overdrive/services/auth-service/resources/db"
	httpadapter "overdrive/services/auth-service/src/adapters/http"
	oauthadapter "overdrive/services/auth-service/src/adapters/oauth"
	prismaadapter "overdrive/services/auth-service/src/adapters/repository/prisma"
	"overdrive/services/auth-service/src/core/usecases"
	"overdrive/shared/authjwt"
	"overdrive/shared/bootstrap"
)

// main serves without argument; `healthcheck` probes the running service (see bootstrap.Main).
// It exits non-zero on failure.
func main() {
	bootstrap.Main("auth-service", run)
}

// run wires the dependencies and serves until SIGINT/SIGTERM. Returning instead of calling
// log.Fatal lets the deferred cleanup (database disconnect) run on every exit path.
func run() error {
	cfg, err := bootstrap.LoadConfig(
		"auth-service",
		"Authentication and token lifecycle service.",
	)
	if err != nil {
		return err
	}
	shutdownTimeout, err := bootstrap.ShutdownTimeout()
	if err != nil {
		return err
	}

	// Required, no insecure fallback: if this were allowed to default to a
	// well-known value, anyone reading this repo could forge valid session tokens.
	jwtSecret := os.Getenv("AUTH_JWT_SECRET")
	if err := authjwt.ValidateSecret(jwtSecret); err != nil {
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

	healthUseCase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	healthController := httpadapter.NewHealthController(healthUseCase)
	sessionRepository := prismaadapter.NewSessionRepository(client)
	sessionUseCase := usecases.NewSessionQueryUseCase(sessionRepository)
	sessionController := httpadapter.NewSessionController(sessionUseCase)
	oauthProviders := oauthadapter.LoadProviders(os.Getenv)
	authUseCase := usecases.NewAuthQueryUseCase(sessionRepository, jwtSecret, oauthProviders...)
	authController := httpadapter.NewAuthController(authUseCase)

	server := bootstrap.NewServer(":"+cfg.HTTPPort, httpadapter.NewRouter(healthController, sessionController, authController, authUseCase), bootstrap.DefaultServerLimits())

	log.Printf("%s listening on: %s", cfg.ServiceName, cfg.HTTPPort)

	if err := bootstrap.Serve(server, shutdownTimeout); err != nil {
		return err
	}
	log.Printf("%s stopped", cfg.ServiceName)
	return nil
}
