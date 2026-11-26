/**
##
## OverDrive 2026
## All Technical rights reserved
##
## main.go - Package main source file for services/ingestion-service.
##
*/

package main

import (
	"log"
	httpdispatcher "overdrive/services/ingestion-service/src/adapters/dispatch/http"
	httpadapter "overdrive/services/ingestion-service/src/adapters/http"
	"overdrive/services/ingestion-service/src/adapters/providers/openf1"
	serviceconfig "overdrive/services/ingestion-service/src/config"
	"overdrive/services/ingestion-service/src/core/usecases"
	"overdrive/shared/bootstrap"
	"time"
)

// liveShutdownTimeout is how long the live jobs get to end once interrupted.
const liveShutdownTimeout = 5 * time.Second

// main serves without argument; `healthcheck` probes the running service (see bootstrap.Main).
// It exits non-zero on failure.
func main() {
	bootstrap.Main("ingestion-service", run)
}

// run wires the dependencies and serves until SIGINT/SIGTERM. Returning instead of calling
// log.Fatal lets the deferred cleanup (database disconnect) run on every exit path.
func run() error {
	cfg, err := serviceconfig.Load()
	if err != nil {
		return err
	}
	shutdownTimeout, err := bootstrap.ShutdownTimeout()
	if err != nil {
		return err
	}

	healthUseCase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	healthController := httpadapter.NewHealthController(healthUseCase)

	provider := openf1.NewClient(cfg.OpenF1BaseURL, cfg.OpenF1Timeout, cfg.OpenF1RetryCount, cfg.OpenF1RetryDelay, openf1.ClientOptions{
		MinInterval:      cfg.OpenF1MinInterval,
		MaxResponseBytes: cfg.OpenF1MaxResponseBytes,
		Username:         cfg.OpenF1Username,
		Password:         cfg.OpenF1Password,
		TokenURL:         cfg.OpenF1TokenURL,
	})
	mapper := openf1.NewMapper()
	dispatcher := httpdispatcher.NewDispatcher(cfg.RaceDataServiceURL, cfg.ChampionshipServiceURL, cfg.InternalIngestionToken, cfg.DispatchTimeout)
	ingestUseCase := usecases.NewIngestOpenF1UseCase(provider, mapper, dispatcher)
	guard := usecases.NewSessionGuard()
	ingestionController := httpadapter.NewIngestionController(usecases.NewGuardedIngestionUseCase(ingestUseCase, guard))
	liveUseCase := usecases.NewLiveIngestionUseCase(provider, ingestUseCase, mapper, dispatcher, guard, usecases.LiveOptions{
		Interval:    cfg.OpenF1LivePollInterval,
		Grace:       cfg.OpenF1LiveGrace,
		MaxFailures: cfg.OpenF1LiveMaxFailures,
		MaxJobs:     cfg.OpenF1LiveMaxJobs,
	})
	liveController := httpadapter.NewLiveController(liveUseCase)
	// Live jobs end on every exit path, including a listener that fails to start.
	defer func() {
		liveUseCase.Close()
		if !liveUseCase.Wait(liveShutdownTimeout) {
			log.Printf("live ingestion jobs still running after %s", liveShutdownTimeout)
		}
	}()

	server := bootstrap.NewServer(":"+cfg.HTTPPort, httpadapter.NewRouter(healthController, ingestionController, liveController), bootstrap.DefaultServerLimits())
	// Jobs stop as soon as the shutdown begins instead of after the in-flight requests.
	server.RegisterOnShutdown(liveUseCase.Close)

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := bootstrap.Serve(server, shutdownTimeout); err != nil {
		return err
	}
	log.Printf("%s stopped", cfg.ServiceName)
	return nil
}
