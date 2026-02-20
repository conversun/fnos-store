package api

type appResponse struct {
	AppName          string `json:"appname"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description,omitempty"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installed_version"`
	LatestVersion    string `json:"latest_version"`
	AvailableVersion string `json:"available_version,omitempty"`
	HasUpdate        bool   `json:"has_update"`
	Platform         string `json:"platform"`
	ReleaseURL       string `json:"release_url"`
	ReleaseNotes     string `json:"release_notes"`
	Status           string `json:"status"`
	ServicePort      int    `json:"service_port,omitempty"`
	Homepage         string `json:"homepage,omitempty"`
	IconURL          string `json:"icon_url,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
	DownloadCount    int    `json:"download_count"`
	AppType          string `json:"app_type,omitempty"`
}

type appsListResponse struct {
	Apps      []appResponse `json:"apps"`
	LastCheck string        `json:"last_check"`
}

type checkResponse struct {
	Status           string `json:"status"`
	Checked          int    `json:"checked"`
	UpdatesAvailable int    `json:"updates_available"`
}

type statusResponse struct {
	Status    string        `json:"status"`
	Busy      bool          `json:"busy"`
	Operation string        `json:"operation"`
	AppName   string        `json:"appname"`
	StartedAt string        `json:"started_at"`
	LastCheck string        `json:"last_check"`
	Platform  string        `json:"platform"`
	ActiveOps []QueueStatus `json:"active_operations,omitempty"`
}

type storeUpdateResponse struct {
	CurrentVersion   string `json:"current_version"`
	AvailableVersion string `json:"available_version,omitempty"`
	HasUpdate        bool   `json:"has_update"`
}

type errorResponse struct {
	Error string `json:"error"`
}
