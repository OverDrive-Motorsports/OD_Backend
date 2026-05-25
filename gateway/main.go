package main

import (
	"errors"
	"log"
	"net/http"
	"overdrive/gateway/internal/app"
	"overdrive/gateway/internal/config"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	runtime, err := app.Build(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := runtime.Close(); closeErr != nil {
			log.Printf("unable to close runtime dependencies: %v", closeErr)
		}
	}()

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           runtime.Handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("gateway listening on :%s", cfg.HTTPPort)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
