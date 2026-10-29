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

// ErrChampionshipUnavailable wraps a failed call to championship-service so a
// usecase mixing the downstream client with a local repository call can still
// tell the controller which side failed (502 vs 500).
var ErrChampionshipUnavailable = errors.New("championship-service unavailable")
