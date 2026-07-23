package diagnostics

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	MaxLogBytes      = 2048
	MaxLogLines      = 80
	MaxErrorMsgBytes = 1024
	MaxURLBytes      = 6144
	IssueRepoURL     = "https://github.com/conversun/fnos-apps/issues/new"
	IssueTemplate    = "bug-report.yml"
)

type DiagnosticReport struct {
	App          string `json:"app"`
	DisplayName  string `json:"display_name"`
	Version      string `json:"version,omitempty"`
	Arch         string `json:"arch"`
	AppType      string `json:"app_type,omitempty"`
	FailedStep   string `json:"failed_step"`
	ErrorMessage string `json:"error_message"`
	LogTail      string `json:"log_tail"`
	LogTruncated bool   `json:"log_truncated"`
	StoreVersion string `json:"store_version"`
	Platform     string `json:"platform"`
	Timestamp    string `json:"timestamp"`
}

func NormalizeArch(goarch string) string {
	switch goarch {
	case "amd64", "x86":
		return "x86"
	case "arm64", "arm":
		return "ARM"
	default:
		return "x86"
	}
}

func TruncateLogTail(raw string, maxLines, maxBytes int) (out string, truncated bool) {
	if raw == "" {
		return "", false
	}

	out = raw
	if maxLines >= 0 {
		lines := strings.Split(out, "\n")
		if len(lines) > maxLines {
			out = strings.Join(lines[len(lines)-maxLines:], "\n")
			truncated = true
		}
	}

	if maxBytes > 0 && len(out) > maxBytes {
		out = validUTF8Suffix(out, maxBytes)
		truncated = true
	}

	return out, truncated
}

func TruncateError(s string) string {
	if len(s) <= MaxErrorMsgBytes {
		return s
	}
	return validUTF8Prefix(s, MaxErrorMsgBytes)
}

func BuildIssueURL(r DiagnosticReport) (string, error) {
	values := url.Values{}
	values.Set("template", IssueTemplate)
	values.Set("app", r.App)
	values.Set("version", r.Version)
	values.Set("arch", r.Arch)
	values.Set("description", issueDescription(r))
	values.Set("logs", r.LogTail)

	issueURL := IssueRepoURL + "?" + values.Encode()
	if len(issueURL) > MaxURLBytes {
		return "", fmt.Errorf("diagnostic issue URL exceeds %d bytes", MaxURLBytes)
	}

	return issueURL, nil
}

func issueDescription(r DiagnosticReport) string {
	return fmt.Sprintf(
		"%s failed during %s: %s\n\nVersion: %s\nArch: %s\nStore Version: %s\nPlatform: %s\nTimestamp: %s",
		r.DisplayName,
		r.FailedStep,
		r.ErrorMessage,
		r.Version,
		r.Arch,
		r.StoreVersion,
		r.Platform,
		r.Timestamp,
	)
}

func validUTF8Suffix(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}

	start := len(s) - maxBytes
	for start < len(s) && !utf8.ValidString(s[start:]) {
		_, size := utf8.DecodeRuneInString(s[start:])
		start += size
	}

	return s[start:]
}

func validUTF8Prefix(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	end := maxBytes
	for end > 0 && !utf8.ValidString(s[:end]) {
		end--
	}

	return s[:end]
}
