package core

import (
	"fnos-store/internal/platform"
	"fnos-store/internal/source"
)

type AppState string

const (
	AppStateNotInstalled AppState = "not_installed"
	AppStateInstalled    AppState = "installed"
	AppStateUpdateReady  AppState = "update_ready"
	AppStateRunning      AppState = "running"
	AppStateStopped      AppState = "stopped"
)

type RegistryApp struct {
	Remote    *source.RemoteApp
	Local     *platform.InstalledApp
	State     AppState
	HasUpdate bool
}

type Registry struct {
	apps map[string]*RegistryApp
}

func NewRegistry() *Registry {
	return &Registry{
		apps: make(map[string]*RegistryApp),
	}
}
