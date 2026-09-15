/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## main.go - Package main source file for services/ingestion-service.
	##
*/
package main

import (
	"errors"
	"log"
	"net/http"
	httpdispatcher "overdrive/services/ingestion-service/src/adapters/dispatch/http"
	httpadapter "overdrive/services/ingestion-service/src/adapters/http"
	"overdrive/services/ingestion-service/src/adapters/providers/openf1"
	serviceconfig "overdrive/services/ingestion-service/src/config"
	"overdrive/services/ingestion-service/src/core/usecases"
	"time"
)

// main bootstraps the service process, wires dependencies, and starts the HTTP server.
func main() {
	cfg, err := serviceconfig.Load()
	if err != nil {
		log.Fatal(err)
	}

	healthUseCase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	healthController := httpadapter.NewHealthController(healthUseCase)

	provider := openf1.NewClient(cfg.OpenF1BaseURL, cfg.OpenF1Timeout, cfg.OpenF1RetryCount, cfg.OpenF1RetryDelay)
	mapper := openf1.NewMapper()
	dispatcher := httpdispatcher.NewDispatcher(cfg.RaceDataServiceURL, cfg.ChampionshipServiceURL, cfg.DispatchTimeout)
	ingestUseCase := usecases.NewIngestOpenF1UseCase(provider, mapper, dispatcher)
	ingestionController := httpadapter.NewIngestionController(ingestUseCase)

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           httpadapter.NewRouter(healthController, ingestionController),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
