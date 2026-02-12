package core

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
