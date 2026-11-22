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
)

// main runs the service and exits non-zero when it fails to start or to stop cleanly.
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
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
	})
	mapper := openf1.NewMapper()
	dispatcher := httpdispatcher.NewDispatcher(cfg.RaceDataServiceURL, cfg.ChampionshipServiceURL, cfg.InternalIngestionToken, cfg.DispatchTimeout)
	ingestUseCase := usecases.NewIngestOpenF1UseCase(provider, mapper, dispatcher)
	ingestionController := httpadapter.NewIngestionController(ingestUseCase)

	server := bootstrap.NewServer(":"+cfg.HTTPPort, httpadapter.NewRouter(healthController, ingestionController), bootstrap.DefaultServerLimits())

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := bootstrap.Serve(server, shutdownTimeout); err != nil {
		return err
	}
	log.Printf("%s stopped", cfg.ServiceName)
	return nil
}
