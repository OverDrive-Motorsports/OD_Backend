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
