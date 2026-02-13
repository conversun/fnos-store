package platform

import "strings"

// parseListTable parses the table output of `appcenter-cli list`.
// Data rows use "│" (U+2502) as column delimiters:
//
//	│ APP NAME │ DISPLAY NAME │ VERSION │ STATUS │ DEPENDENCY APPS │
//	│ emby     │ Emby         │ 4.9.3.0 │ running│                 │
func parseListTable(output string) ([]InstalledApp, error) {
	var apps []InstalledApp
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "│") {
			continue
		}

		cells := splitTableRow(line)
		if len(cells) < 4 {
			continue
		}

		if strings.EqualFold(cells[0], "APP NAME") {
			continue
		}

		apps = append(apps, InstalledApp{
			AppName:     cells[0],
			DisplayName: cells[1],
			Version:     cells[2],
			Status:      cells[3],
		})
	}
	return apps, nil
}

func splitTableRow(line string) []string {
	parts := strings.Split(line, "│")
	var cells []string
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" || len(cells) > 0 {
			cells = append(cells, v)
		}
	}
	for len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}
