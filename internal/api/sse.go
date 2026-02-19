package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type progressPayload struct {
	Step       string `json:"step"`
	Progress   int    `json:"progress,omitempty"`
	Message    string `json:"message,omitempty"`
	NewVersion string `json:"new_version,omitempty"`
	AppName    string `json:"appname,omitempty"`
	Speed      int64  `json:"speed,omitempty"`
	Downloaded int64  `json:"downloaded,omitempty"`
	Total      int64  `json:"total,omitempty"`
}

type sseStream struct {
	w       http.ResponseWriter
	r       *http.Request
	flusher http.Flusher
	appname string
}

func newSSEStream(w http.ResponseWriter, r *http.Request, appname string) (*sseStream, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	return &sseStream{w: w, r: r, flusher: flusher, appname: appname}, nil
}

func (s *sseStream) sendProgress(payload progressPayload) error {
	if err := s.r.Context().Err(); err != nil {
		return err
	}

	payload.AppName = s.appname

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(s.w, "event: progress\ndata: %s\n\n", raw); err != nil {
		return err
	}

	s.flusher.Flush()
	return s.r.Context().Err()
}

func (s *sseStream) sendError(message string) error {
	return s.sendProgress(progressPayload{Step: "error", Message: message})
}
