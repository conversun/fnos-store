package api

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	if len(containers) > 0 {
		s.serveDockerLogs(w, appName, containers, lines)
		return
	}

	s.serveFileLogs(w, appName, lines)
}

func (s *Server) findDockerContainers(appName string) []string {
	composePaths := []string{
		filepath.Join(s.appsDir, appName, "app", "docker", "docker-compose.yaml"),
		filepath.Join(s.appsDir, appName, "app", "docker-compose.yaml"),
	}

	for _, p := range composePaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var containers []string
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "container_name:") {
				name := strings.TrimSpace(strings.TrimPrefix(trimmed, "container_name:"))
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
