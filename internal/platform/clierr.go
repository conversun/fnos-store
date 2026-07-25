package platform

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// appcenter-cli ALWAYS exits 0 — even for hard failures. Measured on
// fnOS 1.2.0203 (appcenter-cli 1.0.1):
//
//	$ appcenter-cli default-volume 2   -> exit 0, prints "0"  (set is a no-op)
//	$ appcenter-cli start nosuchapp    -> exit 0, prints "[Error]Application [nosuchapp] is not installed."
//	$ appcenter-cli install-local ...  -> exit 0, prints "[Error]Something wrong with appcenter: code 20001"
//	$ appcenter-cli default-volume --x -> exit 0, prints "Error: unknown flag: --x"
//
// Trusting the exit status therefore turns every CLI failure into a silent
// success. That is what let an update run install-local (which uninstalls
// before reinstalling), have the reinstall fail, and still report "操作完成"
// to the user with the app destroyed — conversun/fnos-apps#189.
//
// Since the exit code carries no signal, failure has to be read out of the
// output. fnOS prints two distinct error envelopes:
//
//	"[Error]..."  — appcenter's own errors (may be preceded by "[Info]...")
//	"Error: ..."  — cobra flag/arg parsing errors, always at line start
//
// Detection is deliberately restricted to those two exact envelopes. Scanning
// for loose words like "error"/"failed" would misread legitimate output such
// as `check` printing "Not Installed" or `status` printing "noinstall".
const (
	errEnvelopeAppcenter = "[Error]"
	errEnvelopeCobra     = "Error:"
)

// ErrCLIFailure marks a CLI invocation that exited 0 but printed an error
// envelope. Callers can use errors.Is to distinguish it from exec failures.
var ErrCLIFailure = errors.New("appcenter-cli reported a failure")

// cliOutputFailure reports whether output carries an fnOS error envelope.
//
// "[Error]" is matched anywhere in a line, not just at the start, because the
// observed uninstall failure interleaves it with a preceding info tag:
//
//	[Info]Uninstall checking [Error]Application [foo] is not installed.
//
// "Error:" is matched only at line start — that is where cobra emits it, and
// anchoring avoids tripping over an app description that merely contains the
// word.
func cliOutputFailure(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, errEnvelopeAppcenter) {
			return true
		}
		if strings.HasPrefix(line, errEnvelopeCobra) {
			return true
		}
	}
	return false
}

// cliErrorDetail extracts the most informative line of an error envelope so
// the SSE message shown to the user names the real cause (e.g. the fnOS
// error code) instead of a generic failure string.
func cliErrorDetail(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, errEnvelopeAppcenter); idx >= 0 {
			if detail := strings.TrimSpace(line[idx+len(errEnvelopeAppcenter):]); detail != "" {
				return detail
			}
		}
		if strings.HasPrefix(line, errEnvelopeCobra) {
			if detail := strings.TrimSpace(strings.TrimPrefix(line, errEnvelopeCobra)); detail != "" {
				return detail
			}
		}
	}
	return strings.TrimSpace(output)
}

// Known-good outputs per command. Anything outside these vocabularies means
// the CLI drifted or emitted an unrecognized diagnostic, and a mutation must
// not be treated as successful on the strength of output nobody can parse.
const (
	checkInstalled    = "Installed"
	checkNotInstalled = "Not Installed"
)

// validStatuses is the status vocabulary observed from `appcenter-cli list`
// and `appcenter-cli status` on fnOS 1.2.0203. "noinstall" is a legitimate
// status for an absent app and must NOT be read as an error.
var validStatuses = map[string]bool{
	"running":   true,
	"stopped":   true,
	"start":     true,
	"starting":  true,
	"stopping":  true,
	"nostart":   true,
	"noinstall": true,
}

// parseCheckOutput requires the exact vocabulary of `appcenter-cli check`.
// Any other text — an error envelope, a localized string, a future status —
// is rejected instead of silently collapsing to "not installed", which would
// otherwise make a broken CLI look like a clean uninstall.
func parseCheckOutput(out string) (bool, error) {
	switch strings.TrimSpace(out) {
	case checkInstalled:
		return true, nil
	case checkNotInstalled:
		return false, nil
	default:
		return false, fmt.Errorf("%w: check 返回无法识别的输出: %q", ErrCLIFailure, out)
	}
}

// parseStatusOutput requires a known status token.
func parseStatusOutput(out string) (string, error) {
	s := strings.TrimSpace(out)
	if validStatuses[s] {
		return s, nil
	}
	return "", fmt.Errorf("%w: status 返回无法识别的输出: %q", ErrCLIFailure, out)
}

// parseVolumeOutput requires a single integer. The CLI's default-volume
// getter is known to be broken on some fnOS builds (it returns 0 while the
// daemon's own database holds a valid index), so the caller must additionally
// verify the value is a mounted volume — parsing alone proves nothing.
func parseVolumeOutput(out string) (int, error) {
	s := strings.TrimSpace(out)
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: default-volume 返回非数字输出: %q", ErrCLIFailure, out)
	}
	return v, nil
}
