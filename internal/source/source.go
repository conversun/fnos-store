package source

import "context"

// RemoteApp represents an app available from a remote source.
type RemoteApp struct {
	AppName       string
	DisplayName   string
	Version       string
	Description   string
	HomepageURL   string
	UpdatedAt     string
	ReleaseTag    string
	FilePrefix    string
	FpkVersion    string
	ServicePort   int
	Platforms     []string
	FpkURL        string
	IconURL       string
	DownloadCount int
	AppType       string
	Category      string
	Source        string
}

// Source provides access to a remote app catalog.
type Source interface {
	// Name returns the identifier of this source (e.g., "fnos-apps").
	Name() string

	// FetchApps retrieves all available apps from this source.
	FetchApps(ctx context.Context) ([]RemoteApp, error)
}
