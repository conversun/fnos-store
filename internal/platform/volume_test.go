package platform

import "testing"

// TestVolumeIndexForPath locks the path-boundary matching that keeps an app's
// current-volume resolution correct: /vol1 must never swallow /vol10, and the
// deepest matching mount wins. This underpins update volume pinning
// (conversun/fnos-apps#189).
func TestVolumeIndexForPath(t *testing.T) {
	vols := []VolumeInfo{
		{Index: 1, Path: "/vol1"},
		{Index: 10, Path: "/vol10"},
		{Index: 2, Path: "/vol2"},
	}
	cases := []struct {
		name   string
		target string
		want   int
		wantOK bool
	}{
		{"target under vol1 not vol10", "/vol1/@appcenter/emby", 1, true},
		{"data dir under vol10", "/vol10/@appdata/emby", 10, true},
		{"vol1 prefix must not swallow vol10 target", "/vol10/x", 10, true},
		{"exact mount path", "/vol2", 2, true},
		{"unknown volume yields not found", "/mnt/other/emby", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := volumeIndexForPath(tc.target, vols)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Errorf("idx = %d, want %d", got, tc.want)
			}
		})
	}
}
