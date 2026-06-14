/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## main.go - Service entrypoint and HTTP server bootstrap for auth-service.
	##
*/
package main

import (
	"errors"
	"log"
	"net/http"
	httpadapter "overdrive/services/auth-service/src/adapters/http"
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

	usecase := usecases.NewGetHealthUseCase(cfg.ServiceName)
	healthController := httpadapter.NewHealthController(usecase)
	authController := httpadapter.NewAuthController()

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           httpadapter.NewRouter(healthController, authController),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("%s listening on :%s", cfg.ServiceName, cfg.HTTPPort)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
