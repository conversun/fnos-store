package api

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (s *Server) handleGetAppLogs(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("appname")
	if appName == "" {
		writeAPIError(w, http.StatusBadRequest, "missing app name")
		return
	}

	lines, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	if lines <= 0 {
		lines = 200
	}

	containers := s.findDockerContainers(appName)
	tail, _, err := fetchAppLogTail(r.Context(), s.appsDir, appName, lines, 0)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(containers) > 0 {
		writeJSON(w, http.StatusOK, appLogsResponse{
			AppName:    appName,
			LogLines:   splitLogTail(tail),
			Source:     "docker",
			Containers: containers,
		})
		return
	}

	if appLogFileExists(s.appsDir, appName) {
		writeJSON(w, http.StatusOK, appLogsResponse{
			AppName:  appName,
			LogLines: splitFileLogTail(tail),
			Source:   "file",
		})
		return
	}

	writeJSON(w, http.StatusOK, appLogsResponse{
		AppName:  appName,
		LogLines: []string{},
		Source:   "none",
	})
}

func (s *Server) findDockerContainers(appName string) []string {
	return findDockerContainers(s.appsDir, appName)
}

func findDockerContainers(appsDir, appName string) []string {
	composePaths := []string{
		filepath.Join(appsDir, appName, "app", "docker", "docker-compose.yaml"),
		filepath.Join(appsDir, appName, "app", "docker-compose.yaml"),
	}

	for _, p := range composePaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var containers []string
		for line := range strings.SplitSeq(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if name, ok := strings.CutPrefix(trimmed, "container_name:"); ok {
				name := strings.TrimSpace(name)
				name = strings.Trim(name, "\"' ")
				if name != "" {
					containers = append(containers, name)
				}
			}
		}
		if len(containers) > 0 {
			return containers
		}
	}
	return nil
}

func fetchAppLogTail(ctx context.Context, appsDir, app string, maxLines, maxBytes int) (tail string, truncated bool, err error) {
	containers := findDockerContainers(appsDir, app)
	if len(containers) > 0 {
		tail, truncated := fetchDockerLogTail(ctx, containers, maxLines, maxBytes)
		return tail, truncated, nil
	}

	for _, logPath := range appLogPaths(appsDir, app) {
		data, err := os.ReadFile(logPath)
		if err != nil {
			continue
		}
		return tailFileLog(data, maxLines, maxBytes)
	}

	return "", false, nil
}

func fetchDockerLogTail(ctx context.Context, containers []string, maxLines, maxBytes int) (string, bool) {
	var allLines []string
	tail := strconv.Itoa(maxLines)

	for _, cname := range containers {
		cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", tail, cname)
		out, err := cmd.CombinedOutput()
		if err != nil {
			allLines = append(allLines, fmt.Sprintf("=== [%s] 获取日志失败: %v ===", cname, err))
			continue
		}
		if len(containers) > 1 {
			allLines = append(allLines, fmt.Sprintf("=== [%s] ===", cname))
		}
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}
	}

	tailText := strings.Join(allLines, "\n")
	return capTailBytes(tailText, maxBytes)
}

func tailFileLog(data []byte, maxLines, maxBytes int) (string, bool, error) {
	if len(data) == 0 {
		return "", false, nil
	}

	allLines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	truncated := false
	if maxLines > 0 && len(allLines) > maxLines {
		allLines = allLines[len(allLines)-maxLines:]
		truncated = true
	}

	tail := strings.Join(allLines, "\n")
	if capped, byteTruncated := capTailBytes(tail, maxBytes); byteTruncated {
		tail = capped
		truncated = true
	}

	return tail, truncated, nil
}

func capTailBytes(tail string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(tail) <= maxBytes {
		return tail, false
	}

	start := len(tail) - maxBytes
	for start < len(tail) && !utf8.RuneStart(tail[start]) {
		start++
	}
	if start >= len(tail) {
		return "", true
	}

	return tail[start:], true
}

func appLogPaths(appsDir, appName string) []string {
	return []string{
		filepath.Join(appsDir, appName, "var", appName+".log"),
		filepath.Join("/var/log/apps", appName+".log"),
	}
}

func appLogFileExists(appsDir, appName string) bool {
	for _, logPath := range appLogPaths(appsDir, appName) {
		if _, err := os.ReadFile(logPath); err == nil {
			return true
		}
	}
	return false
}

func splitLogTail(tail string) []string {
	if tail == "" {
		return []string{}
	}
	return strings.Split(tail, "\n")
}

func splitFileLogTail(tail string) []string {
	if tail == "" {
		return []string{""}
	}
	return strings.Split(tail, "\n")
}

func (s *Server) serveDockerLogs(w http.ResponseWriter, appName string, containers []string, lines int) {
	var allLines []string
	tail := strconv.Itoa(lines)

	for _, cname := range containers {
		cmd := exec.Command("docker", "logs", "--tail", tail, cname)
		out, err := cmd.CombinedOutput()
		if err != nil {
			allLines = append(allLines, fmt.Sprintf("=== [%s] 获取日志失败: %v ===", cname, err))
			continue
		}
		if len(containers) > 1 {
			allLines = append(allLines, fmt.Sprintf("=== [%s] ===", cname))
		}
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}
	}

	writeJSON(w, http.StatusOK, appLogsResponse{
		AppName:    appName,
		LogLines:   allLines,
		Source:     "docker",
		Containers: containers,
	})
}

func (s *Server) serveFileLogs(w http.ResponseWriter, appName string, lines int) {
	logPaths := []string{
		filepath.Join(s.appsDir, appName, "var", appName+".log"),
		filepath.Join("/var/log/apps", appName+".log"),
	}

	for _, logPath := range logPaths {
		data, err := os.ReadFile(logPath)
		if err != nil {
			continue
		}

		allLines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if len(allLines) > lines {
			allLines = allLines[len(allLines)-lines:]
		}

		writeJSON(w, http.StatusOK, appLogsResponse{
			AppName:  appName,
			LogLines: allLines,
			Source:   "file",
		})
		return
	}

	writeJSON(w, http.StatusOK, appLogsResponse{
		AppName:  appName,
		LogLines: []string{},
		Source:   "none",
	})
}
