package diagnostics

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDiagnosticReportJSONTags(t *testing.T) {
	typ := reflect.TypeFor[DiagnosticReport]()
	cases := map[string]string{
		"App":          `json:"app"`,
		"DisplayName":  `json:"display_name"`,
		"Version":      `json:"version,omitempty"`,
		"Arch":         `json:"arch"`,
		"AppType":      `json:"app_type,omitempty"`,
		"FailedStep":   `json:"failed_step"`,
		"ErrorMessage": `json:"error_message"`,
		"LogTail":      `json:"log_tail"`,
		"LogTruncated": `json:"log_truncated"`,
		"StoreVersion": `json:"store_version"`,
		"Platform":     `json:"platform"`,
		"Timestamp":    `json:"timestamp"`,
	}

	for fieldName, want := range cases {
		field, ok := typ.FieldByName(fieldName)
		if !ok {
			t.Fatalf("missing field %s", fieldName)
		}
		if got := string(field.Tag); got != want {
			t.Errorf("%s tag: got %q, want %q", fieldName, got, want)
		}
	}
}

func TestNormalizeArch(t *testing.T) {
	cases := []struct {
		name   string
		goarch string
		want   string
	}{
		{"amd64 maps to x86", "amd64", "x86"},
		{"arm64 maps to ARM", "arm64", "ARM"},
		{"arm maps to ARM", "arm", "ARM"},
		{"x86 remains x86", "x86", "x86"},
		{"unknown defaults to x86", "riscv64", "x86"},
		{"empty defaults to x86", "", "x86"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeArch(tc.goarch); got != tc.want {
				t.Errorf("NormalizeArch(%q): got %q, want %q", tc.goarch, got, tc.want)
			}
		})
	}
}

func TestTruncateLogTail(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		got, truncated := TruncateLogTail("", MaxLogLines, MaxLogBytes)
		if got != "" || truncated {
			t.Fatalf("got (%q, %v), want empty and not truncated", got, truncated)
		}
	})

	t.Run("under limits", func(t *testing.T) {
		raw := "line 1\nline 2\nline 3"
		got, truncated := TruncateLogTail(raw, MaxLogLines, MaxLogBytes)
		if got != raw || truncated {
			t.Fatalf("got (%q, %v), want original and not truncated", got, truncated)
		}
	})

	t.Run("over line limit keeps last 80 lines", func(t *testing.T) {
		lines := make([]string, 100)
		for i := range lines {
			lines[i] = "line"
		}
		lines[19] = "drop-me"
		lines[20] = "first-kept"
		lines[99] = "last-kept"

		got, truncated := TruncateLogTail(strings.Join(lines, "\n"), MaxLogLines, MaxLogBytes)
		if !truncated {
			t.Fatal("expected truncated=true")
		}
		gotLines := strings.Split(got, "\n")
		if len(gotLines) != MaxLogLines {
			t.Fatalf("kept lines: got %d, want %d", len(gotLines), MaxLogLines)
		}
		if gotLines[0] != "first-kept" || gotLines[len(gotLines)-1] != "last-kept" {
			t.Fatalf("unexpected kept range: first=%q last=%q", gotLines[0], gotLines[len(gotLines)-1])
		}
		if strings.Contains(got, "drop-me") {
			t.Fatalf("old lines were not dropped: %q", got)
		}
	})

	t.Run("over bytes single huge line keeps UTF-8 safe suffix", func(t *testing.T) {
		raw := strings.Repeat("好", 900)
		got, truncated := TruncateLogTail(raw, MaxLogLines, MaxLogBytes)
		if !truncated {
			t.Fatal("expected truncated=true")
		}
		if len(got) > MaxLogBytes {
			t.Fatalf("len(got)=%d exceeds %d", len(got), MaxLogBytes)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("truncated output is not valid UTF-8")
		}
		if !strings.HasSuffix(raw, got) {
			t.Fatalf("got is not a suffix of raw")
		}
	})

	t.Run("exact byte boundary", func(t *testing.T) {
		raw := strings.Repeat("a", MaxLogBytes)
		got, truncated := TruncateLogTail(raw, MaxLogLines, MaxLogBytes)
		if got != raw || truncated {
			t.Fatalf("got len=%d truncated=%v, want exact boundary unchanged", len(got), truncated)
		}
	})
}

func TestTruncateError(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"short", "install failed", "install failed"},
		{"oversize ascii", strings.Repeat("x", MaxErrorMsgBytes+10), strings.Repeat("x", MaxErrorMsgBytes)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TruncateError(tc.in); got != tc.want {
				t.Errorf("got len=%d, want len=%d", len(got), len(tc.want))
			}
		})
	}

	t.Run("multibyte UTF-8 boundary safe", func(t *testing.T) {
		in := strings.Repeat("界", 400)
		got := TruncateError(in)
		if len(got) > MaxErrorMsgBytes {
			t.Fatalf("len(got)=%d exceeds %d", len(got), MaxErrorMsgBytes)
		}
		if !utf8.ValidString(got) {
			t.Fatal("truncated error is not valid UTF-8")
		}
		if !strings.HasPrefix(in, got) {
			t.Fatalf("got is not a prefix of input")
		}
	})
}

func TestBuildIssueURL(t *testing.T) {
	report := DiagnosticReport{
		App:          "plex/&中文",
		DisplayName:  "Plex 媒体库",
		Version:      "1.2.3",
		Arch:         "ARM",
		FailedStep:   "install-fpk",
		ErrorMessage: "安装失败: bad & worse",
		LogTail:      "line 1\nline 2 with spaces & symbols",
		StoreVersion: "0.9.0",
		Platform:     "fnOS 0.9",
		Timestamp:    "2026-06-18T10:11:12Z",
	}

	got, err := BuildIssueURL(report)
	if err != nil {
		t.Fatalf("BuildIssueURL returned error: %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse failed: %v", err)
	}
	if base := parsed.Scheme + "://" + parsed.Host + parsed.Path; base != IssueRepoURL {
		t.Fatalf("base URL: got %q, want %q", base, IssueRepoURL)
	}

	query := parsed.Query()
	if query.Get("template") != IssueTemplate {
		t.Errorf("template: got %q, want %q", query.Get("template"), IssueTemplate)
	}
	if query.Get("app") != report.App {
		t.Errorf("app round-trip: got %q, want %q", query.Get("app"), report.App)
	}
	if query.Get("version") != report.Version {
		t.Errorf("version: got %q, want %q", query.Get("version"), report.Version)
	}
	if query.Get("arch") != report.Arch {
		t.Errorf("arch: got %q, want %q", query.Get("arch"), report.Arch)
	}
	if query.Get("logs") != report.LogTail {
		t.Errorf("logs round-trip: got %q, want %q", query.Get("logs"), report.LogTail)
	}
	if !strings.Contains(parsed.RawQuery, "app=plex%2F%26%E4%B8%AD%E6%96%87") {
		t.Fatalf("special chars were not encoded in app query: %s", parsed.RawQuery)
	}

	description := query.Get("description")
	for _, want := range []string{report.DisplayName, report.FailedStep, report.ErrorMessage, report.Version, report.Arch, report.StoreVersion, report.Platform, report.Timestamp} {
		if !strings.Contains(description, want) {
			t.Errorf("description missing %q: %q", want, description)
		}
	}

	if _, err := url.QueryUnescape(parsed.RawQuery); err != nil {
		t.Fatalf("raw query did not query-unescape: %v", err)
	}
}

func TestBuildIssueURLOversize(t *testing.T) {
	report := DiagnosticReport{
		App:          "huge",
		DisplayName:  "Huge App",
		Version:      "1.0.0",
		Arch:         "x86",
		FailedStep:   "install",
		ErrorMessage: strings.Repeat("e", MaxURLBytes),
		LogTail:      strings.Repeat("l", MaxURLBytes),
		StoreVersion: "0.9.0",
		Platform:     "fnOS",
		Timestamp:    "2026-06-18T10:11:12Z",
	}

	got, err := BuildIssueURL(report)
	if err == nil {
		t.Fatalf("expected oversize URL error, got nil and URL len=%d", len(got))
	}
	if got != "" {
		t.Fatalf("oversize URL should be empty, got %q", got)
	}
}
