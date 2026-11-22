/**
##
## OverDrive 2026
## All Technical rights reserved
##
## identity_test.go - Unit tests pinning the deterministic ingestion IDs.
##
*/

package ingestion

import "testing"

// TestIDFormats pins every ID format. Championship, race-data and the clients join on these
// strings, so any change here silently breaks every cross-service link and every stored row.
func TestIDFormats(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"provider", ProviderID("openf1"), "provider:openf1"},
		{"championship", ChampionshipID("openf1", "f1"), "openf1:championship:f1"},
		{"event", EventID("openf1", 1229), "openf1:event:1229"},
		{"session", SessionID("openf1", 9998), "openf1:session:9998"},
		{"team", TeamID("openf1", "f1", "red-bull-racing"), "openf1:f1:team:red-bull-racing"},
		{"driver", DriverID("openf1", "f1", "63"), "openf1:f1:driver:63"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s ID = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestIDFormats_SanitizeFragments proves every text fragment is trimmed, lower-cased, and has
// spaces and slashes turned into dashes, so "Red Bull Racing" and " red bull racing " give
// the same team ID; numeric keys are written as they are.
func TestIDFormats_SanitizeFragments(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"provider", ProviderID(" OpenF1 "), "provider:openf1"},
		{"championship", ChampionshipID("OpenF1", " F1 "), "openf1:championship:f1"},
		{"event", EventID("OPENF1", 1229), "openf1:event:1229"},
		{"session", SessionID("OpenF1", 9998), "openf1:session:9998"},
		{"team", TeamID("OpenF1", "F1", "Red Bull Racing"), "openf1:f1:team:red-bull-racing"},
		{"team with slash", TeamID("openf1", "f1", "Sauber/Kick"), "openf1:f1:team:sauber-kick"},
		{"driver", DriverID("openf1", "F1", " 63 "), "openf1:f1:driver:63"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s ID = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestIDFormats_EmptyFragmentBecomesUnknown proves a blank fragment is written "unknown"
// instead of leaving an empty segment ("openf1:f1:team:"), which would merge unrelated rows.
func TestIDFormats_EmptyFragmentBecomesUnknown(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"provider", ProviderID("  "), "provider:unknown"},
		{"championship", ChampionshipID("", ""), "unknown:championship:unknown"},
		{"session", SessionID("", 1), "unknown:session:1"},
		{"team", TeamID("openf1", "f1", ""), "openf1:f1:team:unknown"},
		{"driver", DriverID("openf1", "", " "), "openf1:unknown:driver:unknown"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s ID = %q, want %q", c.name, c.got, c.want)
		}
	}
}
