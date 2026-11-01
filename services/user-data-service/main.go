/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## main.go - Service entrypoint and HTTP server bootstrap for user-data-service.
	##
*/
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	db "overdrive/services/user-data-service/resources/db"
	httpadapter "overdrive/services/user-data-service/src/adapters/http"
	prismaadapter "overdrive/services/user-data-service/src/adapters/repository/prisma"
	"overdrive/services/user-data-service/src/core/usecases"
	"overdrive/shared/authjwt"
	"overdrive/shared/bootstrap"
	"time"
)

func main() {
	cfg, err := bootstrap.LoadConfig(
		"user-data-service",
		"User profile, preferences, and saved data service.",
	)
	if err != nil {
		log.Fatal(err)
	}

	// AUTH_JWT_SECRET verifies the users' access tokens (issued by auth-service). Without it no
	// preset request can be authenticated, so the service refuses to start.
	verifier, err := authjwt.NewVerifier(os.Getenv("AUTH_JWT_SECRET"))
	if err != nil {
		log.Fatal("AUTH_JWT_SECRET is required")
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

	usecase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	controller := httpadapter.NewHealthController(usecase)

	presetRepository := prismaadapter.NewPresetRepository(client)
	presetUsecase := usecases.NewPresetUseCase(presetRepository)
	presetController := httpadapter.NewPresetController(presetUsecase)

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           httpadapter.NewRouter(controller, presetController, verifier),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
