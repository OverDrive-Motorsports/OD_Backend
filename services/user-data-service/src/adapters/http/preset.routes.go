/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset.routes.go - Registers the preset routes.
##
*/

package httpadapter

import "net/http"

// registerPresetRoutes attaches the preset handlers. The gateway strips its `/presets` prefix,
// so the public `/presets` and `/presets/{presetId}` arrive here as `/` and `/{presetId}`.
// `GET /health` stays more specific than `GET /{presetId}` and keeps priority.
func registerPresetRoutes(mux *http.ServeMux, controller *PresetController, auth func(http.Handler) http.Handler) {
	handle := func(pattern string, handler http.HandlerFunc, withBody bool) {
		var h http.Handler = handler
		if withBody {
			h = http.MaxBytesHandler(h, presetBodyLimitBytes)
		}
		mux.Handle(pattern, auth(h))
	}
	handle("GET /{$}", controller.List, false)
	handle("POST /{$}", controller.Create, true)
	handle("GET /{presetId}", controller.Get, false)
	handle("PATCH /{presetId}", controller.Update, true)
	handle("DELETE /{presetId}", controller.Delete, false)
}
