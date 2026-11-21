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

// registerPresetRoutes attaches the preset handlers under `/presets`. The gateway forwards that
// prefix unchanged (from the public `/v1/presets` and `/presets`), so a public path can only
// reach preset routes: `/presets/health` is a preset ID (404 PRESET_NOT_FOUND), never the
// service's own `GET /health`. The collection answers with and without a trailing slash.
func registerPresetRoutes(mux *http.ServeMux, controller *PresetController, auth func(http.Handler) http.Handler) {
	handle := func(pattern string, handler http.HandlerFunc, withBody bool) {
		var h http.Handler = handler
		if withBody {
			h = http.MaxBytesHandler(h, presetBodyLimitBytes)
		}
		mux.Handle(pattern, auth(h))
	}
	handle("GET /presets", controller.List, false)
	handle("GET /presets/{$}", controller.List, false)
	handle("POST /presets", controller.Create, true)
	handle("POST /presets/{$}", controller.Create, true)
	handle("GET /presets/{presetId}", controller.Get, false)
	handle("PATCH /presets/{presetId}", controller.Update, true)
	handle("DELETE /presets/{presetId}", controller.Delete, false)
}
