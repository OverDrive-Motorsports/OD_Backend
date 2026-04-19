/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## main.go - Service entrypoint and HTTP server bootstrap for race-data-service.
	##
*/
package main

import (
	"errors"
	"log"
	"net/http"
	httpadapter "overdrive/services/race-data-service/src/adapters/http"
	"overdrive/services/race-data-service/src/core/usecases"
	"overdrive/shared/bootstrap"
	"time"
)

func main() {
	cfg, err := bootstrap.LoadConfig(
		"race-data-service",
		"Race data query and normalized read model service.",
	)
	if err != nil {
		log.Fatal(err)
	}

	usecase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	controller := httpadapter.NewHealthController(usecase)

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           httpadapter.NewRouter(controller),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
