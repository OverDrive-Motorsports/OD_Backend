/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## main.go - Package main source file for services/race-data-service.
	##
*/
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	db "overdrive/services/race-data-service/resources/db"
	championshipclient "overdrive/services/race-data-service/src/adapters/downstream/championship"
	httpadapter "overdrive/services/race-data-service/src/adapters/http"
	prismaadapter "overdrive/services/race-data-service/src/adapters/repository/prisma"
	"overdrive/services/race-data-service/src/core/usecases"
	"overdrive/shared/bootstrap"
	"time"
)

// main bootstraps the service process, wires dependencies, and starts the HTTP server.
func main() {
	cfg, err := bootstrap.LoadConfig(
		"race-data-service",
		"Race data query and normalized read model service.",
	)
	if err != nil {
		log.Fatal(err)
	}

	client := db.NewClient()
	if err := client.Prisma.Connect(); err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := client.Prisma.Disconnect(); err != nil {
			log.Printf("failed to disconnect prisma client: %v", err)
		}
	}()

	healthUsecase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	healthController := httpadapter.NewHealthController(healthUsecase)
	championshipURL := os.Getenv("CHAMPIONSHIP_SERVICE_URL")
	if championshipURL == "" {
		championshipURL = "http://localhost:3003"
	}
	queryRepository := prismaadapter.NewQueryRepository(client)
	championshipClient := championshipclient.New(championshipURL, 5*time.Second)
	sessionUsecase := usecases.NewSessionQueryUseCase(queryRepository, championshipClient)
	sessionController := httpadapter.NewSessionController(sessionUsecase)

	// raceControlBroadcaster is a single-instance, in-memory pub/sub backing the
	// POST /sessions/{sessionId}/race/control long-poll. See
	// src/core/usecases/race_control_broadcaster.go for the horizontal-scale caveat.
	raceControlBroadcaster := usecases.NewRaceControlBroadcaster()
	ingestionRepository := prismaadapter.NewIngestionRepository(client)
	ingestionUsecase := usecases.NewAcceptIngestionBatchUseCase(cfg.ServiceName, ingestionRepository, raceControlBroadcaster)
	ingestionController := httpadapter.NewIngestionController(ingestionUsecase)

	raceLiveRepository := prismaadapter.NewRaceLiveRepository(client)
	raceLiveUsecase := usecases.NewRaceLiveUseCase(raceLiveRepository, championshipClient, raceControlBroadcaster)
	raceLiveController := httpadapter.NewRaceLiveController(raceLiveUsecase)

	telemetryRepository := prismaadapter.NewTelemetryRepository(client)
	telemetryUsecase := usecases.NewTelemetryUseCase(telemetryRepository, championshipClient)
	telemetryController := httpadapter.NewTelemetryController(telemetryUsecase)

	server := &http.Server{
		Addr: ":" + cfg.HTTPPort,
		Handler: httpadapter.NewRouter(
			healthController,
			ingestionController,
			sessionController,
			raceLiveController,
			telemetryController,
		),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
