package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

func TestFetchAppLogTail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cases := []struct {
		name          string
		app           string
		content       *string
		maxLines      int
		maxBytes      int
		wantTail      string
		wantTruncated bool
	}{
		{
			name:     "reads app var log under apps dir",
			app:      "demo",
			content:  strPtr("alpha\nbeta\n"),
			maxLines: 10,
			wantTail: "alpha\nbeta",
		},
		{
			name:          "keeps last max lines",
			app:           "demo",
			content:       strPtr("one\ntwo\nthree\nfour\n"),
			maxLines:      2,
			wantTail:      "three\nfour",
			wantTruncated: true,
		},
		{
			name:          "caps max bytes on utf8 rune boundary",
			app:           "demo",
			content:       strPtr("prefix\n你好世界"),
			maxLines:      10,
			maxBytes:      8,
			wantTail:      "世界",
			wantTruncated: true,
		},
		{
			name:     "missing log returns empty tail",
			app:      "missing",
			maxLines: 10,
			wantTail: "",
		},
		{
			name:     "empty log returns empty tail",
			app:      "demo",
			content:  strPtr(""),
			maxLines: 10,
			wantTail: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			appsDir := t.TempDir()
			if tc.content != nil {
				logDir := filepath.Join(appsDir, tc.app, "var")
				if err := os.MkdirAll(logDir, 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				logPath := filepath.Join(logDir, tc.app+".log")
				if err := os.WriteFile(logPath, []byte(*tc.content), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			}

			gotTail, gotTruncated, err := fetchAppLogTail(ctx, appsDir, tc.app, tc.maxLines, tc.maxBytes)
			if err != nil {
				t.Fatalf("fetchAppLogTail returned error: %v", err)
			}
			if gotTail != tc.wantTail {
				t.Errorf("tail: got %q, want %q", gotTail, tc.wantTail)
			}
			if gotTruncated != tc.wantTruncated {
				t.Errorf("truncated: got %v, want %v", gotTruncated, tc.wantTruncated)
			}
			if tc.maxBytes > 0 && len(gotTail) > tc.maxBytes {
				t.Errorf("tail length %d exceeds maxBytes %d", len(gotTail), tc.maxBytes)
			}
			if !utf8.ValidString(gotTail) {
				t.Errorf("tail is not valid UTF-8: %q", gotTail)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
