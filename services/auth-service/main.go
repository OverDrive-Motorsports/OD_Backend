/**
##
## OverDrive 2026
## All Technical rights reserved
##
## catalog.service.go - Package ports source file for services/championship-service/src/core/ports.
##
*/

package main

import (
	"errors"
	"log"
	"net/http"
	db "overdrive/services/auth-service/resources/db"
	httpadapter "overdrive/services/auth-service/src/adapters/http"
	prismaadapter "overdrive/services/auth-service/src/adapters/repository/prisma"
	"overdrive/services/auth-service/src/core/usecases"
	"overdrive/shared/bootstrap"
	"time"
)

func main() {
	cfg, err := bootstrap.LoadConfig(
		"auth-service",
		"Authentication and token lifecycle service.",
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

	healthUseCase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	healthController := httpadapter.NewHealthController(healthUseCase)
	sessionRepository := prismaadapter.NewSessionRepository(client)
	sessionUseCase := usecases.NewSessionQueryUseCase(sessionRepository)
	sessionController := httpadapter.NewSessionController(sessionUseCase)

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           httpadapter.NewRouter(healthController, sessionController),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
