package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"fnos-store/internal/core"
	"fnos-store/internal/diagnostics"
	"fnos-store/internal/source"
)

func TestHandleGetAppDiagnostic(t *testing.T) {
	const appName = "jellyfin"

	t.Run("known app returns report and issue URL", func(t *testing.T) {
		s := newDiagnosticTestServer(t)
		resp := requestDiagnostic(t, s, appName, "step=installing&error=permission%20denied")

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var got diagnosticResponse
		decodeResponse(t, resp, &got)

		if got.Report.App != appName {
			t.Fatalf("Report.App = %q, want %q", got.Report.App, appName)
		}
		if got.Report.FailedStep != "installing" {
			t.Fatalf("Report.FailedStep = %q, want installing", got.Report.FailedStep)
		}
		if got.Report.Arch != "x86" && got.Report.Arch != "ARM" {
			t.Fatalf("Report.Arch = %q, want x86 or ARM", got.Report.Arch)
		}
		if got.IssueURL == "" {
			t.Fatal("IssueURL is empty")
		}
		if _, err := url.Parse(got.IssueURL); err != nil {
			t.Fatalf("IssueURL is not parseable: %v", err)
		}
	})

	t.Run("unknown app uses slug fallback", func(t *testing.T) {
		s := newDiagnosticTestServer(t)
		resp := requestDiagnostic(t, s, "missing-app", "step=downloading")

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var got diagnosticResponse
		decodeResponse(t, resp, &got)

		if got.Report.DisplayName != "missing-app" {
			t.Fatalf("Report.DisplayName = %q, want slug fallback", got.Report.DisplayName)
		}
		if got.Report.Version != "" {
			t.Fatalf("Report.Version = %q, want empty", got.Report.Version)
		}
	})

	t.Run("missing step returns bad request", func(t *testing.T) {
		s := newDiagnosticTestServer(t)
		resp := requestDiagnostic(t, s, appName, "error=permission%20denied")

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
		}
	})

	t.Run("oversize error is truncated", func(t *testing.T) {
		s := newDiagnosticTestServer(t)
		resp := requestDiagnostic(t, s, appName, "step=installing&error="+url.QueryEscape(strings.Repeat("x", diagnostics.MaxErrorMsgBytes+512)))

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var got diagnosticResponse
		decodeResponse(t, resp, &got)

		if len(got.Report.ErrorMessage) > diagnostics.MaxErrorMsgBytes {
			t.Fatalf("len(Report.ErrorMessage) = %d, want <= %d", len(got.Report.ErrorMessage), diagnostics.MaxErrorMsgBytes)
		}
	})

	t.Run("missing log file still returns diagnostic", func(t *testing.T) {
		s := newDiagnosticTestServer(t)
		resp := requestDiagnostic(t, s, appName, "step=verifying")

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var got diagnosticResponse
		decodeResponse(t, resp, &got)

		if got.Report.LogTail != "" {
			t.Fatalf("Report.LogTail = %q, want empty", got.Report.LogTail)
		}
		if got.Report.LogTruncated {
			t.Fatal("Report.LogTruncated = true, want false")
		}
		if got.IssueURL == "" {
			t.Fatal("IssueURL is empty")
		}
		if _, err := url.Parse(got.IssueURL); err != nil {
			t.Fatalf("IssueURL is not parseable: %v", err)
		}
	})
}

func newDiagnosticTestServer(t *testing.T) *Server {
	t.Helper()

	registry := core.NewRegistry()
	registry.Merge([]core.Manifest{{AppName: "fnos-apps-store", Version: "1.0.0"}}, []source.RemoteApp{
		{
			AppName:     "jellyfin",
			DisplayName: "Jellyfin",
			Version:     "10.10.7",
			FpkVersion:  "10.10.7-fpk1",
			AppType:     "native",
		},
		{
			AppName:     "fnos-apps-store",
			DisplayName: "fnOS Apps",
			Version:     "1.0.0",
		},
	}, nil)

	return &Server{
		registry: registry,
		appsDir:  t.TempDir(),
		storeApp: "fnos-apps-store",
	}
}

func requestDiagnostic(t *testing.T, s *Server, appName, rawQuery string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/apps/"+appName+"/diagnostic?"+rawQuery, nil)
	req.SetPathValue("appname", appName)
	rec := httptest.NewRecorder()
	s.handleGetAppDiagnostic(rec, req)
	return rec
}

func decodeResponse(t *testing.T, resp *httptest.ResponseRecorder, out any) {
	t.Helper()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, resp.Body.String())
	}
}
