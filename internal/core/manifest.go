package core

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Manifest struct {
	AppName        string
	Version        string
	DisplayName    string
	Platform       string
	Maintainer     string
	MaintainerURL  string
	Distributor    string
	DistributorURL string
	ServicePort    int
	Description    string
	Source         string
	Checksum       string
}

const (
	manifestFieldWidth      = 16
	conversunDistributorTag = "conversun"
)

func ParseManifest(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open manifest %q: %w", path, err)
	}
	defer f.Close()

	m := &Manifest{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		key, value, ok := parseManifestLine(line)
		if !ok {
			continue
		}

		switch key {
		case "appname":
			m.AppName = value
		case "version":
			m.Version = value
		case "display_name":
			m.DisplayName = value
		case "platform":
			m.Platform = value
		case "maintainer":
			m.Maintainer = value
		case "maintainer_url":
			m.MaintainerURL = value
		case "distributor":
			m.Distributor = value
		case "distributor_url":
			m.DistributorURL = value
		case "service_port":
			if value == "" {
				continue
			}
			p, parseErr := strconv.Atoi(value)
			if parseErr != nil {
				return nil, fmt.Errorf("parse service_port from %q: %w", path, parseErr)
			}
			m.ServicePort = p
		case "desc":
			m.Description = value
		case "source":
			m.Source = value
		case "checksum":
			m.Checksum = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan manifest %q: %w", path, err)
	}

	return m, nil
}

func ScanInstalled(appsDir string) ([]Manifest, error) {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, fmt.Errorf("read apps dir %q: %w", appsDir, err)
	}

	apps := make([]Manifest, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(appsDir, entry.Name(), "manifest")
		m, parseErr := ParseManifest(manifestPath)
		if parseErr != nil {
			if errors.Is(parseErr, os.ErrNotExist) {
				continue
			}
			return nil, parseErr
		}

		if m.Distributor != conversunDistributorTag {
			continue
		}

		apps = append(apps, *m)
	}

	return apps, nil
}

func parseManifestLine(line string) (key, value string, ok bool) {
	if strings.HasPrefix(line, "#") {
		return "", "", false
	}

	if len(line) > manifestFieldWidth {
		left := line[:manifestFieldWidth]
		right := strings.TrimSpace(line[manifestFieldWidth:])
		if strings.HasPrefix(right, "=") {
			return strings.TrimSpace(left), strings.TrimSpace(strings.TrimPrefix(right, "=")), true
		}
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}
