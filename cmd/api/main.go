/**
##
## OverDrive 2026
## All Technical rights reserved
##
## main.go - API backend entrypoint and graceful shutdown orchestration.
##
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"overdrive/internal/api"
	"overdrive/internal/app"
	"overdrive/internal/config"
)

// main boots the API server and handles graceful startup and shutdown.
func main() {
	cfg := config.Load()

	var (
		addr           = flag.String("addr", cfg.APIAddr, "HTTP listen address")
		defaultYear    = flag.Int("year", 2025, "Default year for /getrace")
		defaultCountry = flag.String("country", "Australia", "Default country for /getrace")
		defaultMeeting = flag.String("meeting", "Australian Grand Prix", "Default meeting for /getrace")
		defaultDriver  = flag.Int("driver-number", 63, "Default driver_number for /getrace (0 = all)")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	server := app.NewHTTPServer(
		cfg,
		api.RaceDefaults{
			Year:         *defaultYear,
			CountryName:  *defaultCountry,
			MeetingName:  *defaultMeeting,
			DriverNumber: *defaultDriver,
		},
		logger,
		*addr,
	)

	fmt.Printf("API listening on %s\n", *addr)
	fmt.Println("Endpoints:")
	fmt.Println("- GET  /health")
	fmt.Println("- GET  /api/v1/race/getrace?year=2025&country=Australia&meeting=Australian+Grand+Prix&driver_number=63")
	fmt.Println("- POST /api/v1/race/sendrace")
	fmt.Println("- GET  /api/v1/race/cache")
	fmt.Println("- GET  /api/v1/race/championship/drivers")
	fmt.Println("- GET  /api/v1/race/championship/constructors")
	fmt.Println("- GET  /api/v1/race/weather")
	fmt.Println("- GET  /api/v1/race/facts")
	fmt.Println("- GET  /api/v1/race/driver?driver_number=63")
	fmt.Println("- GET  /api/v1/race/standings/race")
	fmt.Println("- GET  /api/v1/race/video-url")
	fmt.Println("- Legacy: /getrace and /sendrace kept for compatibility")

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	<-sigCtx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "graceful shutdown failed: %v\n", err)
		os.Exit(1)
	}
}
