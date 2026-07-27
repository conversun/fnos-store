//go:build linux

package platform

import (
	"encoding/json"
	"errors"
	"testing"
)

// The daemon answers HTTP 200 even for failures, putting the real outcome in
// the body's code field. Treating a non-zero code as success would repeat
// exactly the mistake that made appcenter-cli's exit status meaningless.
func TestDaemonErrorCarriesCode(t *testing.T) {
	err := error(&DaemonError{Path: routeUpdateInfo, Code: daemonCodePackageMissing})

	var de *DaemonError
	if !errors.As(err, &de) {
		t.Fatal("DaemonError must be recoverable with errors.As")
	}
	if de.Code != daemonCodePackageMissing {
		t.Errorf("code = %d, want %d", de.Code, daemonCodePackageMissing)
	}
	if de.Error() == "" {
		t.Error("error message must not be empty")
	}
}

// The message should surface the daemon's own text when it provides one — for
// example 19000 names the missing wizard field, which is the only way a user
// can tell what to fill in.
func TestDaemonErrorPrefersDaemonMessage(t *testing.T) {
	withMsg := &DaemonError{Path: routeInstallTask, Code: daemonCodeWizardRequired,
		Msg: "wizard required field wizard_natfrp_token not found"}
	if got := withMsg.Error(); got == "" || !contains(got, "wizard_natfrp_token") {
		t.Errorf("error %q should include the daemon's message", got)
	}

	noMsg := &DaemonError{Path: routeUpdateTask, Code: daemonCodeValidation}
	if got := noMsg.Error(); !contains(got, routeUpdateTask) {
		t.Errorf("error %q should name the route when the daemon gives no message", got)
	}
}

// These constants encode measured daemon behavior; a typo would silently
// change control flow (e.g. DaemonUpgradeAvailable keys on 10030).
func TestMeasuredDaemonConstants(t *testing.T) {
	if daemonCodeValidation != 10030 {
		t.Errorf("validation code = %d, want 10030 (measured)", daemonCodeValidation)
	}
	if daemonCodePackageMissing != 10100 {
		t.Errorf("package-missing code = %d, want 10100 (measured)", daemonCodePackageMissing)
	}
	if daemonCodeWizardRequired != 19000 {
		t.Errorf("wizard-required code = %d, want 19000 (measured)", daemonCodeWizardRequired)
	}
	if daemonStatusSuccess != 2 || daemonStatusRunning != 1 {
		t.Errorf("task status constants drifted: running=%d success=%d", daemonStatusRunning, daemonStatusSuccess)
	}
	if daemonSocket != "/var/run/com.trim.app.center.sock" {
		t.Errorf("socket path = %q", daemonSocket)
	}
}

// WizardParam must serialize as {"key","value"}. The daemon rejects
// paramKey/paramValue and name/value with 10030 — measured against sakurafrp.
func TestWizardParamJSONShape(t *testing.T) {
	b, err := json.Marshal([]WizardParam{{Key: "wizard_token", Value: "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	want := `[{"key":"wizard_token","value":"secret"}]`
	if got != want {
		t.Errorf("wizard params serialized as %s, want %s", got, want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
