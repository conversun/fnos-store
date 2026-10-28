package api

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// cleanupAppNameRe constrains what handleCleanup will touch on disk. The
// handler runs as root and deletes directories, so the name must be a plain
// identifier: leading alnum, then alnum/dot/dash/underscore only — no
// separators, no traversal, no globs (conversun/fnos-apps#312).
var cleanupAppNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// residuePaths lists every on-disk location a half-registered app can leave
// behind: the /varApps symlink tree, the volume payload/data dirs, and the
// daemon's never-reaped staging copies. Patterns are fixed prefixes with the
// validated appname embedded — never a user-supplied path — mirroring
// removeStagedPackage's discipline.
func residuePaths(appsDir, appname string) []string {
	patterns := []string{
		filepath.Join(appsDir, appname),
		"/vol*/@appcenter/" + appname,
		"/vol*/@appdata/" + appname,
		"/vol*/appcenter-downloads/" + appname + "*-tpk",
	}
	var out []string
	for _, pattern := range patterns {
		if matches, err := filepath.Glob(pattern); err == nil {
			out = append(out, matches...)
		}
	}
	return out
}

// handleCleanup removes the residue of a half-registered app: the daemon has
// no record of it (uninstall refuses with 10300, stop reports 未安装) while the
// on-disk leftovers keep the app showing as installed — the exact dead-end
// users hit in #295/#306/#312. One click replaces the manual SSH recipe.
func (s *Server) handleCleanup(w http.ResponseWriter, r *http.Request) {
	appname := r.PathValue("appname")
	if !cleanupAppNameRe.MatchString(appname) {
		writeAPIError(w, http.StatusBadRequest, "无效的应用名")
		return
	}

	if !s.queue.TryStart("cleanup", appname) {
		writeAPIError(w, http.StatusConflict, "another operation is already running")
		return
	}
	defer s.queue.FinishApp(appname)

	// Safety gate: cleanup is only for apps app center no longer knows. A
	// REGISTERED app must go through the normal uninstall so the daemon can
	// run its hooks (and keep data the user may want).
	var registered bool
	if err := s.queue.WithCLI(func() error {
		list, listErr := s.ac.List()
		if listErr != nil {
			return listErr
		}
		for _, app := range list {
			if app.AppName == appname {
				registered = true
				break
			}
		}
		return nil
	}); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "无法确认应用注册状态，已取消清理: "+err.Error())
		return
	}
	if registered {
		writeAPIError(w, http.StatusConflict, "应用仍在 app center 注册中，请使用卸载功能；清理残留仅用于卸载失败的残留状态")
		return
	}

	removed := make([]string, 0, 4)
	for _, path := range residuePaths(s.appsDir, appname) {
		if err := os.RemoveAll(path); err == nil {
			removed = append(removed, path)
		}
	}

	// Best-effort container removal: half-registered docker apps can leave a
	// container behind that no daemon task will ever clean up.
	dockerNote := ""
	if _, err := exec.LookPath("docker"); err == nil {
		if out, err := exec.Command("docker", "rm", "-f", appname).CombinedOutput(); err == nil {
			dockerNote = strings.TrimSpace(string(out))
		}
	}

	if s.cacheStore != nil {
		s.cacheStore.RemoveInstalledTag(appname)
	}

	if err := s.refreshRegistry(r.Context()); err != nil {
		writeJSON(w, http.StatusOK, cleanupResponse{Removed: removed, DockerNote: dockerNote, Warning: "残留已清理，但刷新应用列表失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cleanupResponse{Removed: removed, DockerNote: dockerNote})
}

// daemonMissing reports whether the daemon's installed list is loaded and
// does not contain appname — the half-registered state. An empty map means
// the daemon was unreachable at the last refresh; that must not paint every
// installed app as residue.
func (s *Server) daemonMissing(appname string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.statusByApp) == 0 {
		return false
	}
	_, ok := s.statusByApp[appname]
	return !ok
}
