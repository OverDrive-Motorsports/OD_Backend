/**
##
## OverDrive 2026
## All Technical rights reserved
##
## errors.go - Package domain source file for services/ingestion-service/src/core/domain.
##
*/

package domain

import "errors"

// ErrUnsupportedResource is returned by the OpenF1 client/mapper when asked to fetch or map a
// resource name that is not part of the supported OpenF1 resource set. It is a domain-level
// sentinel (rather than a raw string-matched error) so adapters/http can classify it with
// errors.Is instead of parsing the error message.
var ErrUnsupportedResource = errors.New("unsupported OpenF1 resource")

// ErrIngestionFailed is returned when every requested resource failed: nothing was ingested, so
// the request fails as a whole (502) instead of answering with a result made only of failures.
var ErrIngestionFailed = errors.New("every OpenF1 resource failed")

// ErrNoOpenF1Data is returned when OpenF1 has no row at all for the requested resources: usually
// an unknown meeting or session key (OpenF1 answers 404 "No results found" for it) (404).
var ErrNoOpenF1Data = errors.New("OpenF1 has no data for this meeting and session")

// ErrResponseTooLarge is returned by the OpenF1 provider when a response exceeds its size limit.
var ErrResponseTooLarge = errors.New("OpenF1 response exceeds the size limit")
