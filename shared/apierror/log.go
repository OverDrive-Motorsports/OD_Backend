/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## log.go - Package apierror source file for shared/apierror.
	##
*/

package apierror

import (
	"fmt"
	"os"
)

// detail returns the full technical error text for logging, falling back to the client message
// when no wrapped error was set.
func (e *Error) detail() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

// LogDebug prints a single plain-text debug line to stderr:
// status=<code> code=<CODE> path=<path> err=<full detail>
func (e *Error) LogDebug(path string) {
	fmt.Fprintf(os.Stderr, "status=%d code=%s path=%s err=%s\n", e.Status, e.Code, path, e.detail())
}
