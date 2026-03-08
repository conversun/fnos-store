package api

import "net/http"

func (s *Server) handleListRecommended(w http.ResponseWriter, r *http.Request) {
	apps := s.listRecommendedApps()
	respApps := make([]recommendedAppResponse, 0, len(apps))
	for _, app := range apps {
		respApps = append(respApps, recommendedAppResponse{
			Name:          app.Name,
			DisplayName:   app.DisplayName,
			Description:   app.Description,
			IconURL:       app.IconURL,
			SourceURL:     app.SourceURL,
			GitHubRepo:    app.GitHubRepo,
			LatestVersion: app.LatestVersion,
			UpdatedAt:     app.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, recommendedListResponse{Apps: respApps})
}
