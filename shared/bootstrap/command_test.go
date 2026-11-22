/**
##
## OverDrive 2026
## All Technical rights reserved
##
## command_test.go - Unit tests for the command line dispatch of the service binaries.
##
*/

package bootstrap

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
)

// TestDispatch_NoArgumentServes proves a binary without argument runs serve and returns its
// error.
func TestDispatch_NoArgumentServes(t *testing.T) {
	isolateConfig(t)
	errServe := errors.New("served")
	if err := Dispatch("svc", nil, func() error { return errServe }); !errors.Is(err, errServe) {
		t.Fatalf("err = %v, want serve's error", err)
	}
}

// TestDispatch_Healthcheck proves `healthcheck` probes HTTP_PORT without serving, reads
// HTTP_PORT from the .env file like the server does, and names the service in its error.
func TestDispatch_Healthcheck(t *testing.T) {
	dir := isolateConfig(t)
	port := healthServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	writeFile(t, dir, ".env", "HTTP_PORT="+port+"\n")
	serve := func() error { t.Fatal("serve must not run"); return nil }
	if err := Dispatch("svc", []string{"healthcheck"}, serve); err != nil {
		t.Fatalf("healthcheck: %v", err)
	}

	writeFile(t, dir, ".env", "HTTP_PORT=x\n")
	t.Setenv("HTTP_PORT", "")
	if err := os.Unsetenv("HTTP_PORT"); err != nil {
		t.Fatalf("unset: %v", err)
	}
	if err := Dispatch("svc", []string{"healthcheck"}, serve); err == nil || !strings.Contains(err.Error(), "svc healthcheck") {
		t.Fatalf("err = %v, want the probe error naming the service", err)
	}
}

// TestDispatch_InvalidEnvFile proves a malformed .env stops the probe.
func TestDispatch_InvalidEnvFile(t *testing.T) {
	dir := isolateConfig(t)
	writeFile(t, dir, ".env", "NOT_A_PAIR\n")
	if err := Dispatch("svc", []string{"healthcheck"}, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "invalid .env line") {
		t.Fatalf("err = %v, want the invalid .env line error", err)
	}
}

// TestDispatch_UnknownCommand proves any other argument is refused without serving.
func TestDispatch_UnknownCommand(t *testing.T) {
	isolateConfig(t)
	serve := func() error { t.Fatal("serve must not run"); return nil }
	for _, args := range [][]string{{"migrate"}, {"healthcheck", "extra"}} {
		if err := Dispatch("svc", args, serve); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("args %q: err = %v, want the unknown command error", args, err)
		}
	}
}

// TestMain_ServesWithoutArgument proves Main reads os.Args: without argument it runs serve and
// returns normally when serve succeeds.
func TestMain_ServesWithoutArgument(t *testing.T) {
	isolateConfig(t)
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = []string{"svc"}
	served := false
	Main("svc", func() error { served = true; return nil })
	if !served {
		t.Fatal("serve did not run")
	}
}
