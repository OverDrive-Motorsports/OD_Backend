/**
##
## OverDrive 2026
## All Technical rights reserved
##
## server.go - Dependency wiring and HTTP server construction for runtime.
##
*/

package app

import (
	"log/slog"
	"net/http"
	"time"

	"overdrive/internal/api"
	"overdrive/internal/config"
	"overdrive/internal/database"
	"overdrive/internal/providers/openf1"
	repositoryprisma "overdrive/internal/repository/prisma"
	"overdrive/internal/service"
	"overdrive/internal/usecase"
)

// NewHTTPServer builds the production HTTP server with all runtime dependencies wired.
func NewHTTPServer(cfg config.Config, defaults api.RaceDefaults, logger *slog.Logger, addr string) *http.Server {
	client := openf1.NewClient(
		cfg.OpenF1BaseURL,
		cfg.HTTPTimeout,
		cfg.RequestInterval,
		cfg.MaxRetries,
		cfg.RetryDelay,
	)

	builder := usecase.NewRaceBuilder(client)
	store := repositoryprisma.NewRaceArchiveStore(database.PrismaClient)
	raceService := service.NewRaceService(builder, store)
	raceHandler := api.NewRaceHandler(raceService, defaults, cfg.GetRaceTimeout)
	router := api.NewRouter(raceHandler, logger)

	return &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0,
	}
}
