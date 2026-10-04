/**
##
## OverDrive 2026
## All Technical rights reserved
##
## errors.go - Package domain source file for services/race-data-service/src/core/domain.
##
*/

package domain

import "errors"

// ErrUnknownDataset is returned by the query repository when GetSessionDataset or
// GetDriverDataset is called with a dataset name that does not map to any known table. It is a
// domain-level sentinel (rather than a raw string-matched error) so adapters/http can classify
// it with errors.Is instead of parsing the error message.
var ErrUnknownDataset = errors.New("unknown dataset")

// ErrInvalidFeed is returned by ValidateFeeds when a feed list violates the
// contract (unknown provider, missing/forbidden field, bad URL, too many
// entries, duplicate). The wrapped message names the failing index and field;
// adapters/http classifies it with errors.Is and answers 400.
var ErrInvalidFeed = errors.New("invalid feed")

// ErrSessionNotFound and ErrDriverNotFound are returned by the driver feed
// usecase when championship-service does not know the session, or the driver
// is not part of the session's driver list. Distinct sentinels let the
// controller answer SESSION_NOT_FOUND vs DRIVER_NOT_FOUND without inspecting
// error strings.
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrDriverNotFound  = errors.New("driver not found")
)

// ErrChampionshipUnavailable wraps a failed call to championship-service so a
// usecase mixing the downstream client with a local repository call can still
// tell the controller which side failed (502 vs 500).
var ErrChampionshipUnavailable = errors.New("championship-service unavailable")
