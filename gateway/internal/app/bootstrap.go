// Package app wires gateway dependencies into a runnable runtime.

package app

import (
	"net/http"
	httpinbound "overdrive/gateway/internal/adapters/inbound/http"
	"overdrive/gateway/internal/adapters/outbound/auth"
	"overdrive/gateway/internal/config"
	"overdrive/gateway/internal/core/usecases"
)

type Runtime struct {
	Handler http.Handler
	Close   func() error
}

func Build(cfg config.Config) (Runtime, error) {
	tokenValidator := auth.NewStaticTokenValidator(cfg.AuthToken)
	healthUseCase := usecases.NewCheckHealthUseCase()
	authorizeUseCase := usecases.NewAuthorizeRequestUseCase(tokenValidator)

	handler, err := httpinbound.NewHandler(
		cfg.Routes,
		cfg.ServiceHealthRoutes,
		cfg.RateLimitRPS,
		cfg.RateLimitBurst,
		healthUseCase,
		authorizeUseCase,
	)
	if err != nil {
		return Runtime{}, err
	}

	return Runtime{Handler: handler, Close: func() error { return nil }}, nil
}
