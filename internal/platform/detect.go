package platform

import "runtime"

func DetectPlatform() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86"
	case "arm64":
		return "arm"
	default:
		return "x86"
	}
}
