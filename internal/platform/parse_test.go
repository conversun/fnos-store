package platform

import (
	"testing"
)

func TestParseListTable(t *testing.T) {
	input := `┌──────────────────┬──────────────┬──────────────┬─────────┬─────────────────┐
│     APP NAME     │ DISPLAY NAME │   VERSION    │ STATUS  │ DEPENDENCY APPS │
├──────────────────┼──────────────┼──────────────┼─────────┼─────────────────┤
│ fnos-apps-store  │ fnOS Apps    │ 1.0.0        │ running │                 │
│ plexmediaserver  │ Plex         │ 1.43.0.10492 │ running │                 │
│ embyserver       │ Emby         │ 4.9.3.0      │ running │                 │
│ xunlei           │ 迅雷         │ 1.0.2        │ stopped │                 │
└──────────────────┴──────────────┴──────────────┴─────────┴─────────────────┘`

	apps, err := parseListTable(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(apps) != 4 {
		t.Fatalf("expected 4 apps, got %d", len(apps))
	}

	cases := []struct {
		idx         int
		appName     string
		displayName string
		version     string
		status      string
	}{
		{0, "fnos-apps-store", "fnOS Apps", "1.0.0", "running"},
		{1, "plexmediaserver", "Plex", "1.43.0.10492", "running"},
		{2, "embyserver", "Emby", "4.9.3.0", "running"},
		{3, "xunlei", "迅雷", "1.0.2", "stopped"},
	}

	for _, tc := range cases {
		app := apps[tc.idx]
		if app.AppName != tc.appName {
			t.Errorf("[%d] AppName: got %q, want %q", tc.idx, app.AppName, tc.appName)
		}
		if app.DisplayName != tc.displayName {
			t.Errorf("[%d] DisplayName: got %q, want %q", tc.idx, app.DisplayName, tc.displayName)
		}
		if app.Version != tc.version {
			t.Errorf("[%d] Version: got %q, want %q", tc.idx, app.Version, tc.version)
		}
		if app.Status != tc.status {
			t.Errorf("[%d] Status: got %q, want %q", tc.idx, app.Status, tc.status)
		}
	}
}

func TestParseListTableEmpty(t *testing.T) {
	apps, err := parseListTable("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
}

func TestSplitTableRow(t *testing.T) {
	row := "│ embyserver       │ Emby         │ 4.9.3.0      │ running │                 │"
	cells := splitTableRow(row)
	if len(cells) != 4 {
		t.Fatalf("expected 4 cells, got %d: %v", len(cells), cells)
	}
	if cells[0] != "embyserver" {
		t.Errorf("cell[0]: got %q, want %q", cells[0], "embyserver")
	}
	if cells[3] != "running" {
		t.Errorf("cell[3]: got %q, want %q", cells[3], "running")
	}
}
