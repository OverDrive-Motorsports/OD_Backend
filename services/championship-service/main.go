/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## main.go - Package main source file for services/championship-service.
	##
*/
package main

import (
	"errors"
	"log"
	"net/http"
	db "overdrive/services/championship-service/resources/db"
	httpadapter "overdrive/services/championship-service/src/adapters/http"
	prismaadapter "overdrive/services/championship-service/src/adapters/repository/prisma"
	"overdrive/services/championship-service/src/core/usecases"
	"overdrive/shared/bootstrap"
	"time"
)

// main bootstraps the service process, wires dependencies, and starts the HTTP server.
func main() {
	cfg, err := bootstrap.LoadConfig(
		"championship-service",
		"Championship metadata, seasons, and calendar service.",
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
	ingestionRepository := prismaadapter.NewIngestionRepository(client)
	ingestionUsecase := usecases.NewAcceptIngestionBatchUseCase(cfg.ServiceName, ingestionRepository)
	ingestionController := httpadapter.NewIngestionController(ingestionUsecase)
	catalogRepository := prismaadapter.NewCatalogRepository(client)
	catalogUsecase := usecases.NewCatalogQueryUseCase(catalogRepository)
	catalogController := httpadapter.NewCatalogController(catalogUsecase)

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           httpadapter.NewRouter(healthController, ingestionController, catalogController),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
