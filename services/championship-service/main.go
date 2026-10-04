/**
##
## OverDrive 2026
## All Technical rights reserved
##
## main.go - Package main source file for services/championship-service.
##
*/

package main

import (
	"fmt"
	"log"
	"os"
	db "overdrive/services/championship-service/resources/db"
	httpadapter "overdrive/services/championship-service/src/adapters/http"
	prismaadapter "overdrive/services/championship-service/src/adapters/repository/prisma"
	"overdrive/services/championship-service/src/core/usecases"
	"overdrive/shared/bootstrap"
)

// main serves without argument; `healthcheck` probes the running service (see bootstrap.Main).
// It exits non-zero on failure.
func main() {
	bootstrap.Main("championship-service", run)
}

// run wires the dependencies and serves until SIGINT/SIGTERM. Returning instead of calling
// log.Fatal lets the deferred cleanup (database disconnect) run on every exit path.
func run() error {
	cfg, err := bootstrap.LoadConfig(
		"championship-service",
		"Championship metadata, seasons, and calendar service.",
	)
	if err != nil {
		return err
	}
	shutdownTimeout, err := bootstrap.ShutdownTimeout()
	if err != nil {
		return err
	}

	// INTERNAL_INGESTION_TOKEN protects POST /internal/ingestion/batches: only ingestion-service
	// knows it. Refusing to start without it keeps the write endpoint from ever being open.
	internalToken := os.Getenv("INTERNAL_INGESTION_TOKEN")
	if err := httpadapter.ValidateInternalToken(internalToken); err != nil {
		return fmt.Errorf("INTERNAL_INGESTION_TOKEN: %w (generate one with: openssl rand -hex 32)", err)
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

	healthUsecase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	healthController := httpadapter.NewHealthController(healthUsecase)
	ingestionRepository := prismaadapter.NewIngestionRepository(client)
	ingestionUsecase := usecases.NewAcceptIngestionBatchUseCase(cfg.ServiceName, ingestionRepository)
	ingestionController := httpadapter.NewIngestionController(ingestionUsecase)
	catalogRepository := prismaadapter.NewCatalogRepository(client)
	catalogUsecase := usecases.NewCatalogQueryUseCase(catalogRepository)
	catalogController := httpadapter.NewCatalogController(catalogUsecase)
	sessionFeedRepository := prismaadapter.NewSessionFeedRepository(client)
	sessionFeedUsecase := usecases.NewSessionFeedUseCase(sessionFeedRepository)
	sessionFeedController := httpadapter.NewSessionFeedController(sessionFeedUsecase)

	server := bootstrap.NewServer(":"+cfg.HTTPPort, httpadapter.NewRouter(healthController, ingestionController, catalogController, sessionFeedController, internalToken), bootstrap.DefaultServerLimits())

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := bootstrap.Serve(server, shutdownTimeout); err != nil {
		return err
	}
	log.Printf("%s stopped", cfg.ServiceName)
	return nil
}
