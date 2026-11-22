/**
##
## OverDrive 2026
## All Technical rights reserved
##
## command.go - Command line of the service binaries: serve, or probe the running service.
##
*/

package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
)

// HealthcheckCommand makes a binary probe the service running on HTTP_PORT (see Healthcheck)
// instead of serving: the container health check of images without a shell nor curl.
const HealthcheckCommand = "healthcheck"

// Main runs serve when the binary has no argument, the health probe for `healthcheck`, and
// exits non-zero when it fails. serve keeps its own configuration loading.
func Main(serviceName string, serve func() error) {
	if err := Dispatch(serviceName, os.Args[1:], serve); err != nil {
		log.Fatal(err)
	}
}

// Dispatch is Main without the exit. For `healthcheck` the .env files are loaded (optional, as
// for the service) so HTTP_PORT resolves like it does for the server. Any other argument is an
// error.
func Dispatch(serviceName string, args []string, serve func() error) error {
	switch {
	case len(args) == 0:
		return serve()
	case len(args) == 1 && args[0] == HealthcheckCommand:
		if err := LoadEnvFiles(serviceName); err != nil {
			return err
		}
		if err := Healthcheck(context.Background(), os.Getenv("HTTP_PORT")); err != nil {
			return fmt.Errorf("%s %s: %w", serviceName, HealthcheckCommand, err)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q: run %s without argument to serve, or with %q",
			args, serviceName, HealthcheckCommand)
	}
}
