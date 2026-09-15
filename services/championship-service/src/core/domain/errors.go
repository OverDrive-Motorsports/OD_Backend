/**
##
## OverDrive 2026
## All Technical rights reserved
##
## errors.go - Package domain source file for services/championship-service/src/core/domain.
##
*/

package domain

import "errors"

// ErrUnknownDataset is returned by the catalog repository when GetSessionDataset is
// called with a dataset name that does not map to any known table. It is a domain-level
// sentinel (rather than a raw string-matched error) so adapters/http can classify it with
// errors.Is instead of parsing the error message.
var ErrUnknownDataset = errors.New("unknown dataset")
