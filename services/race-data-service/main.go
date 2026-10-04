/**
##
## OverDrive 2026
## All Technical rights reserved
##
## main.go - Package main source file for services/race-data-service.
##
*/

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	db "overdrive/services/race-data-service/resources/db"
	championshipclient "overdrive/services/race-data-service/src/adapters/downstream/championship"
	httpadapter "overdrive/services/race-data-service/src/adapters/http"
	prismaadapter "overdrive/services/race-data-service/src/adapters/repository/prisma"
	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/usecases"
	"overdrive/shared/bootstrap"
	"strconv"
	"time"
)

// main serves without argument; `healthcheck` probes the running service (see bootstrap.Main).
// It exits non-zero on failure.
func main() {
	bootstrap.Main("race-data-service", run)
}

// run wires the dependencies and serves until SIGINT/SIGTERM. Returning instead of calling
// log.Fatal lets the deferred cleanup (poller stop, database disconnect) run on every exit path.
func run() error {
	cfg, err := bootstrap.LoadConfig(
		"race-data-service",
		"Race data query and normalized read model service.",
	)
	if err != nil {
		return err
	}
	shutdownTimeout, err := bootstrap.ShutdownTimeout()
	if err != nil {
		return err
	}
	limits, err := loadRuntimeLimits()
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
	championshipURL := os.Getenv("CHAMPIONSHIP_SERVICE_URL")
	if championshipURL == "" {
		championshipURL = "http://localhost:3003"
	}
	queryRepository := prismaadapter.NewQueryRepository(client)
	championshipClient := championshipclient.New(championshipURL, 5*time.Second)
	sessionUsecase := usecases.NewSessionQueryUseCase(queryRepository, championshipClient)
	sessionController := httpadapter.NewSessionController(sessionUsecase)

	// The race control long poll works across instances: batches go through the
	// RaceControlNotification table, polled by every instance (shared_race_control_broadcaster.go).
	raceControlBroadcaster := usecases.NewSharedRaceControlBroadcaster(
		prismaadapter.NewRaceControlNotificationRepository(client),
		usecases.NewRaceControlBroadcasterWithLimits(limits.raceControlMaxWaiters, limits.raceControlMaxWaitersPerSession),
	)
	pollCtx, stopPolling := context.WithCancel(context.Background())
	defer stopPolling()
	go raceControlBroadcaster.Run(pollCtx)

	replayCache := prismaadapter.NewReplayCacheRepository(client)
	ingestionRepository := prismaadapter.NewIngestionRepository(client)
	ingestionUsecase := usecases.NewAcceptIngestionBatchUseCase(cfg.ServiceName, ingestionRepository, raceControlBroadcaster, replayCache)
	ingestionController := httpadapter.NewIngestionController(ingestionUsecase)

	raceLiveRepository := prismaadapter.NewRaceLiveRepository(client)
	raceLiveUsecase := usecases.NewRaceLiveUseCase(raceLiveRepository, championshipClient, raceControlBroadcaster)
	raceLiveController := httpadapter.NewRaceLiveController(raceLiveUsecase)

	telemetryRepository := prismaadapter.NewTelemetryRepository(client)
	telemetryUsecase := usecases.NewTelemetryUseCase(telemetryRepository, championshipClient)
	telemetryController := httpadapter.NewTelemetryController(telemetryUsecase)

	raceStreamRepository := prismaadapter.NewRaceStreamRepository(client)
	raceReplayStreamUsecase := usecases.NewRaceReplayStreamUseCase(raceStreamRepository, replayCache, championshipClient, limits.replayMaxConcurrent, limits.replayCacheMaxConcurrent)
	raceReplayStreamController := httpadapter.NewRaceReplayStreamController(raceReplayStreamUsecase)

	replayCatalogRepository := prismaadapter.NewReplayCatalogRepository(client)
	replayCatalogUsecase := usecases.NewReplayCatalogUseCase(championshipClient, replayCatalogRepository)
	replayCatalogController := httpadapter.NewReplayCatalogController(replayCatalogUsecase)

	driverFeedRepository := prismaadapter.NewDriverFeedRepository(client)
	driverFeedUsecase := usecases.NewDriverFeedUseCase(driverFeedRepository, championshipClient)
	driverFeedController := httpadapter.NewDriverFeedController(driverFeedUsecase)

	server := bootstrap.NewServer(":"+cfg.HTTPPort, httpadapter.NewRouter(
		healthController,
		ingestionController,
		sessionController,
		raceLiveController,
		telemetryController,
		raceReplayStreamController,
		replayCatalogController,
		driverFeedController,
		internalToken,
	), bootstrap.DefaultServerLimits())
	// Long polls end at once on shutdown instead of holding it for up to 30 s.
	server.RegisterOnShutdown(raceControlBroadcaster.Close)

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := bootstrap.Serve(server, shutdownTimeout); err != nil {
		return err
	}
	log.Printf("%s stopped", cfg.ServiceName)
	return nil
}

// runtimeLimits are the per-instance capacity limits, overridable from the environment.
type runtimeLimits struct {
	raceControlMaxWaiters           int
	raceControlMaxWaitersPerSession int
	replayMaxConcurrent             int
	replayCacheMaxConcurrent        int
}

// loadRuntimeLimits reads RACE_CONTROL_MAX_WAITERS, RACE_CONTROL_MAX_WAITERS_PER_SESSION,
// REPLAY_MAX_CONCURRENT and REPLAY_CACHE_MAX_CONCURRENT, falling back to the defaults when unset.
func loadRuntimeLimits() (runtimeLimits, error) {
	var limits runtimeLimits
	var err error
	if limits.raceControlMaxWaiters, err = positiveIntEnv("RACE_CONTROL_MAX_WAITERS", usecases.DefaultRaceControlMaxWaiters); err != nil {
		return limits, err
	}
	if limits.raceControlMaxWaitersPerSession, err = positiveIntEnv("RACE_CONTROL_MAX_WAITERS_PER_SESSION", usecases.DefaultRaceControlMaxWaitersPerSession); err != nil {
		return limits, err
	}
	if limits.replayMaxConcurrent, err = positiveIntEnv("REPLAY_MAX_CONCURRENT", domain.DefaultReplayMaxConcurrent); err != nil {
		return limits, err
	}
	if limits.replayCacheMaxConcurrent, err = positiveIntEnv("REPLAY_CACHE_MAX_CONCURRENT", domain.DefaultReplayCacheMaxConcurrent); err != nil {
		return limits, err
	}
	return limits, nil
}

// positiveIntEnv reads a positive integer variable, or returns fallback when it is unset.
func positiveIntEnv(name string, fallback int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", name, raw)
	}
	return value, nil
}
