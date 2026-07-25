package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func (s *Server) handleUninstall(w http.ResponseWriter, r *http.Request) {
	appname := r.PathValue("appname")
	if appname == "" {
		writeAPIError(w, http.StatusBadRequest, "appname is required")
		return
	}

	if !s.queue.TryStart("uninstall", appname) {
		writeAPIError(w, http.StatusConflict, "another operation is already running")
		return
	}
	defer s.queue.FinishApp(appname)

	stream, err := newSSEStream(w, r, appname)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Stop is best-effort: an app that is already stopped (or whose service
	// entry is gone) must not block the uninstall the user asked for. The
	// error is surfaced only if the uninstall itself then fails.
	_ = stream.sendProgress(progressPayload{Step: "stopping", Message: "正在停止..."})
	stopErr := s.queue.WithCLI(func() error { return s.ac.Stop(appname) })

	_ = stream.sendProgress(progressPayload{Step: "uninstalling", Message: "正在卸载..."})
	if err := s.queue.WithCLI(func() error { return s.ac.Uninstall(appname) }); err != nil {
		if stopErr != nil {
			_ = stream.sendError(fmt.Sprintf("%v（停止阶段也失败: %v）", err, stopErr))
			return
		}
		_ = stream.sendError(err.Error())
		return
	}

	// appcenter-cli exits 0 even when uninstall fails, so confirm the app is
	// really gone before telling the user it was removed. Without this a failed
	// uninstall still reported 卸载完成 and dropped the cache tag, leaving the
	// UI and the system disagreeing about what is installed.
	// Only a definitive "not there" proves removal. Treating ANY stat error as
	// success would let a permission or transient I/O error masquerade as a
	// completed uninstall.
	if _, err := os.Stat(filepath.Join(s.appsDir, appname, "manifest")); err == nil {
		_ = stream.sendError("卸载未生效：应用仍存在于系统中。请在应用中心手动卸载")
		return
	} else if !os.IsNotExist(err) {
		_ = stream.sendError(fmt.Sprintf("卸载结果无法确认（%v）。请在应用中心确认应用是否已移除", err))
		return
	}

	if s.cacheStore != nil {
		s.cacheStore.RemoveInstalledTag(appname)
	}

	if err := s.refreshRegistry(r.Context()); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "done", Message: "卸载完成"})
}
