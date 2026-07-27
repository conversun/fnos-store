package platform

import (
	"strings"
	"testing"
)

// 1.2.0203 is the build measured to destroy an app on update (error 10237).
func TestUpgradeUnsafeVersionsCoversMeasuredBuild(t *testing.T) {
	reason, found := upgradeUnsafeVersions["1.2.0203"]
	if !found {
		t.Fatal("fnOS 1.2.0203 must be listed as unsafe: updates there delete the app and its data")
	}
	if !strings.Contains(reason, "10237") {
		t.Errorf("reason %q should name the error code so users can match it to their logs", reason)
	}
	if !strings.Contains(reason, "删除") {
		t.Errorf("reason %q should state the data-loss risk plainly", reason)
	}
}

// A deny-list, not an allow-list: refusing every untested build would block
// updates on systems where they work today. Only confirmed-destructive builds
// are listed.
func TestUpgradeUnsafeVersionsIsNarrow(t *testing.T) {
	for _, v := range []string{"1.2.0204", "1.3.0", "0.9.9", ""} {
		if _, blocked := upgradeUnsafeVersions[v]; blocked {
			t.Errorf("version %q is blocked without having been measured as unsafe", v)
		}
	}
}
