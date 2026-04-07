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
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"overdrive/internal/database"
	"strings"
	"sync"
	"syscall"
	"time"

	"overdrive/internal/api"
	"overdrive/internal/app"
	"overdrive/internal/config"
)

// main boots the API server and handles graceful startup and shutdown.
func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogFormat)

	err := database.Connect()
	if err != nil {
		logger.Error("database connection failed",
			"type", "startup",
			"component", "database",
			"error", err.Error(),
		)
		os.Exit(1)
	}
	if database.IsConnected() {
		logger.Info("database connected",
			"type", "startup",
			"component", "database",
		)
	} else {
		logger.Error("database health check failed",
			"type", "startup",
			"component", "database",
		)
		os.Exit(1)
	}

	var (
		addr           = flag.String("addr", cfg.APIAddr, "HTTP listen address")
		defaultYear    = flag.Int("year", 2025, "Default year for /getrace")
		defaultCountry = flag.String("country", "Australia", "Default country for /getrace")
		defaultMeeting = flag.String("meeting", "Australian Grand Prix", "Default meeting for /getrace")
		defaultDriver  = flag.Int("driver-number", 0, "Default driver_number for /getrace (0 = all)")
	)
	flag.Parse()

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

	logger.Info("api server configured",
		"type", "startup",
		"component", "http_server",
		"addr", *addr,
	)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server stopped unexpectedly",
				"type", "runtime",
				"component", "http_server",
				"error", err.Error(),
			)
			os.Exit(1)
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	<-sigCtx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	logger.Info("shutdown signal received",
		"type", "shutdown",
		"component", "http_server",
		"timeout", cfg.ShutdownTimeout.String(),
	)

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed",
			"type", "shutdown",
			"component", "http_server",
			"error", err.Error(),
		)
		os.Exit(1)
	}

	logger.Info("disconnecting database",
		"type", "shutdown",
		"component", "database",
	)
	if database.IsConnected() {
		err := database.Disconnect()
		if err != nil {
			logger.Error("database disconnect failed",
				"type", "shutdown",
				"component", "database",
				"error", err.Error(),
			)
			os.Exit(1)
		}
	}
	logger.Info("shutdown complete",
		"type", "shutdown",
	)
}

// newLogger builds the shared structured logger used across the API process.
func newLogger(format string) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "timestamp"
				if ts, ok := attr.Value.Any().(time.Time); ok {
					attr.Value = slog.StringValue(ts.UTC().Format(time.RFC3339))
				}
			case slog.MessageKey:
				attr.Key = "message"
			case slog.LevelKey:
				attr.Value = slog.StringValue(strings.ToUpper(attr.Value.String()))
			}
			return attr
		},
	}

	if strings.EqualFold(strings.TrimSpace(format), "json") {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}

	return slog.New(newPrettyJSONHandler(os.Stdout, opts))
}

type prettyJSONHandler struct {
	out    io.Writer
	opts   *slog.HandlerOptions
	attrs  []slog.Attr
	groups []string
	mu     *sync.Mutex
}

func newPrettyJSONHandler(out io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return &prettyJSONHandler{
		out:  out,
		opts: opts,
		mu:   &sync.Mutex{},
	}
}

func (h *prettyJSONHandler) Enabled(_ context.Context, level slog.Level) bool {
	min := slog.LevelInfo
	if h.opts != nil && h.opts.Level != nil {
		min = h.opts.Level.Level()
	}
	return level >= min
}

func (h *prettyJSONHandler) Handle(_ context.Context, r slog.Record) error {
	payload := make(map[string]any)

	builtins := []slog.Attr{
		slog.Time(slog.TimeKey, r.Time),
		slog.Any(slog.LevelKey, r.Level),
		slog.String(slog.MessageKey, r.Message),
	}
	for _, attr := range builtins {
		h.appendAttr(payload, nil, attr)
	}

	for _, attr := range h.attrs {
		h.appendAttr(payload, h.groups, attr)
	}
	r.Attrs(func(attr slog.Attr) bool {
		h.appendAttr(payload, h.groups, attr)
		return true
	})

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err = h.out.Write(append(body, '\n'))
	return err
}

func (h *prettyJSONHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *prettyJSONHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := *h
	next.groups = append(append([]string{}, h.groups...), name)
	return &next
}

func (h *prettyJSONHandler) appendAttr(payload map[string]any, groups []string, attr slog.Attr) {
	if h.opts != nil && h.opts.ReplaceAttr != nil {
		attr = h.opts.ReplaceAttr(groups, attr)
	}
	if attr.Key == "" {
		return
	}

	value := attr.Value
	if value.Kind() == slog.KindGroup {
		nestedGroups := groups
		if attr.Key != "" {
			nestedGroups = append(append([]string{}, groups...), attr.Key)
		}
		for _, child := range value.Group() {
			h.appendAttr(payload, nestedGroups, child)
		}
		return
	}

	target := payload
	for _, group := range groups {
		existing, ok := target[group]
		if !ok {
			child := make(map[string]any)
			target[group] = child
			target = child
			continue
		}
		child, ok := existing.(map[string]any)
		if !ok {
			child = make(map[string]any)
			target[group] = child
		}
		target = child
	}

	target[attr.Key] = slogValueToAny(value)
}

func slogValueToAny(value slog.Value) any {
	switch value.Kind() {
	case slog.KindAny:
		return value.Any()
	case slog.KindBool:
		return value.Bool()
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindString:
		return value.String()
	case slog.KindTime:
		return value.Time().UTC().Format(time.RFC3339)
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindGroup:
		child := make(map[string]any)
		for _, attr := range value.Group() {
			if attr.Key == "" {
				continue
			}
			child[attr.Key] = slogValueToAny(attr.Value)
		}
		return child
	default:
		return value.Any()
	}
}
