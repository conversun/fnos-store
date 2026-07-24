package platform

import (
	"path/filepath"
	"strings"
)

// volumeIndexForPath returns the index of the deepest mounted volume whose path
// contains target, using path-boundary matching so /vol1 never matches /vol10.
func volumeIndexForPath(target string, volumes []VolumeInfo) (int, bool) {
	target = filepath.Clean(target)
	bestIdx, bestLen := 0, -1
	for _, v := range volumes {
		vp := filepath.Clean(v.Path)
		if target != vp && !strings.HasPrefix(target, vp+string(filepath.Separator)) {
			continue
		}
		if len(vp) > bestLen {
			bestIdx, bestLen = v.Index, len(vp)
		}
	}
	return bestIdx, bestLen >= 0
}
