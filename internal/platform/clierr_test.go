package platform

import (
	"errors"
	"strings"
	"testing"
)

// TestCLIOutputFailure locks the exit-code-is-useless contract. Every "want
// true" string below was captured verbatim from appcenter-cli 1.0.1 on fnOS
// 1.2.0203 exiting with status 0 — trusting that status is what turned a
// failed reinstall into a reported success (conversun/fnos-apps#189).
//
// The "want false" cases matter just as much: `check` and `status` answer with
// plain words that must never be mistaken for errors, or a healthy system
// would start reporting phantom failures.
func TestCLIOutputFailure(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		// Measured failures (all exited 0).
		{"install-local failure code", "[Error]Something wrong with appcenter: code 20001", true},
		{"start missing app", "[Error]Application [nosuchapp] is not installed.", true},
		{"stop missing app", "[Error]The Application [nosuchapp] is not installed.", true},
		{
			// The envelope is mid-line here, after an [Info] tag — a prefix-only
			// match would miss it entirely.
			"uninstall error after info tag",
			"[Info]Uninstall checking [Error]Application [nosuchapp] is not installed.",
			true,
		},
		{"cobra unknown flag", "Error: unknown flag: --bogus\nUsage:\n  appcenter-cli default-volume", true},
		{"volume hint", "[Error]Use `--volume` to specify the volume index", true},
		{
			"error on a later line",
			"\\ Verifying files.| Verifying files.\n[Error]Install failed. Copy dir /nonexistent to tmp dir",
			true,
		},

		// Legitimate output that MUST NOT be read as failure.
		{"check installed", "Installed", false},
		{"check not installed", "Not Installed", false},
		{"status noinstall", "noinstall", false},
		{"status running", "running", false},
		{"default-volume value", "1", false},
		{"empty", "", false},
		{"progress spinner", "\\ installing.| installing./ installing.- installing.", false},
		{
			// A table row mentioning an app whose name contains "error" is not
			// an error envelope.
			"list table row",
			"│ error-tracker │ Error Tracker │ 1.0.0 │ running │",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cliOutputFailure(tt.output); got != tt.want {
				t.Errorf("cliOutputFailure(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

// TestCLIErrorDetail checks the user-facing message names the real cause
// (e.g. the fnOS error code) rather than a generic failure string.
func TestCLIErrorDetail(t *testing.T) {
	tests := []struct {
		output string
		want   string
	}{
		{"[Error]Something wrong with appcenter: code 20001", "Something wrong with appcenter: code 20001"},
		{"[Info]Uninstall checking [Error]Application [foo] is not installed.", "Application [foo] is not installed."},
		{"Error: unknown flag: --bogus", "unknown flag: --bogus"},
	}
	for _, tt := range tests {
		if got := cliErrorDetail(tt.output); got != tt.want {
			t.Errorf("cliErrorDetail(%q) = %q, want %q", tt.output, got, tt.want)
		}
	}
}

// TestParseCheckOutput requires the exact vocabulary. Unrecognized output must
// error rather than collapse to "not installed", which would make a broken CLI
// look like a clean uninstall.
func TestParseCheckOutput(t *testing.T) {
	t.Run("installed", func(t *testing.T) {
		got, err := parseCheckOutput("Installed")
		if err != nil || !got {
			t.Fatalf("got (%v, %v), want (true, nil)", got, err)
		}
	})
	t.Run("not installed", func(t *testing.T) {
		got, err := parseCheckOutput("Not Installed")
		if err != nil || got {
			t.Fatalf("got (%v, %v), want (false, nil)", got, err)
		}
	})
	t.Run("rejects unrecognized output", func(t *testing.T) {
		if _, err := parseCheckOutput("something else entirely"); err == nil {
			t.Fatal("expected error for unrecognized output, got nil")
		} else if !errors.Is(err, ErrCLIFailure) {
			t.Errorf("error %v should wrap ErrCLIFailure", err)
		}
	})
}

func TestParseStatusOutput(t *testing.T) {
	// "noinstall" is a legitimate status for an absent app, not an error.
	for _, s := range []string{"running", "stopped", "start", "nostart", "noinstall"} {
		if got, err := parseStatusOutput(s); err != nil || got != s {
			t.Errorf("parseStatusOutput(%q) = (%q, %v), want (%q, nil)", s, got, err, s)
		}
	}
	if _, err := parseStatusOutput("[Error]boom"); err == nil {
		t.Error("expected error for an error envelope, got nil")
	}
}

func TestParseVolumeOutput(t *testing.T) {
	if got, err := parseVolumeOutput("2"); err != nil || got != 2 {
		t.Errorf("parseVolumeOutput(\"2\") = (%d, %v), want (2, nil)", got, err)
	}
	// 0 parses fine — it is a syntactically valid answer. Rejecting an invalid
	// volume is the caller's job, since only it knows which volumes are mounted.
	if got, err := parseVolumeOutput("0"); err != nil || got != 0 {
		t.Errorf("parseVolumeOutput(\"0\") = (%d, %v), want (0, nil)", got, err)
	}
	if _, err := parseVolumeOutput("Error: nope"); err == nil {
		t.Error("expected error for non-numeric output, got nil")
	} else if !strings.Contains(err.Error(), "default-volume") {
		t.Errorf("error %v should name the command", err)
	}
}
