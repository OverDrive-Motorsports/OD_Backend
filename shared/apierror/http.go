/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## http.go - Package apierror source file for shared/apierror.
	##
*/

package apierror

import (
	"encoding/json"
	"net/http"
)

// responseBody is the JSON envelope sent to the client for every Error.
type responseBody struct {
	Error struct {
		Code    Code   `json:"code"`
		Status  Status `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

// Write logs the error's technical detail and sends the standardized JSON error response to the
// client. It replaces each service's ad-hoc writeJSON(w, status, map[string]any{...}) call so the
// envelope stays identical across services.
func Write(w http.ResponseWriter, path string, e *Error) {
	e.LogDebug(path)

	var resp responseBody
	resp.Error.Code = e.Code
	resp.Error.Status = e.Status
	resp.Error.Message = e.Message

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(int(e.Status))
	_ = json.NewEncoder(w).Encode(resp)
}
