package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/wentf9/subitohost/internal/engine"
	"github.com/wentf9/subitohost/internal/midi"
)

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"audio_backend": s.engine.Config().Audio.Backend,
		"sample_rate":   s.engine.Config().Audio.SampleRate,
		"buffer_size":   s.engine.Config().Audio.BufferSize,
		"gain":          s.engine.Gain(),
	}
	state := s.engine.State()
	if state != nil {
		resp["current_index"] = state.Index()
		resp["current_profile"] = state.Current().Name
		resp["setlist_name"] = state.Setlist().Name
		resp["total_profiles"] = state.Len()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetSetlist(w http.ResponseWriter, r *http.Request) {
	state := s.engine.State()
	if state == nil {
		writeError(w, http.StatusNotFound, "no setlist loaded")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"setlist":       state.Setlist(),
		"current_index": state.Index(),
	})
}

func (s *Server) handleLoadSetlist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.engine.LoadSetlist(req.Path, 0); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleNext(w http.ResponseWriter, r *http.Request) {
	if s.engine.State() == nil {
		writeError(w, http.StatusBadRequest, "no setlist loaded")
		return
	}
	if err := s.engine.NextProfile(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"current_index": s.engine.State().Index(),
		"name":          s.engine.State().Current().Name,
	})
}

func (s *Server) handlePrev(w http.ResponseWriter, r *http.Request) {
	if s.engine.State() == nil {
		writeError(w, http.StatusBadRequest, "no setlist loaded")
		return
	}
	if err := s.engine.PrevProfile(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"current_index": s.engine.State().Index(),
		"name":          s.engine.State().Current().Name,
	})
}

func (s *Server) handleGoTo(w http.ResponseWriter, r *http.Request) {
	if s.engine.State() == nil {
		writeError(w, http.StatusBadRequest, "no setlist loaded")
		return
	}
	var req struct {
		Index int `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.engine.GoToProfile(req.Index); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"current_index": s.engine.State().Index(),
		"name":          s.engine.State().Current().Name,
	})
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := midi.ListDevices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID int `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.engine.ConnectMIDI(req.DeviceID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

func (s *Server) handleGetGain(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]float64{"gain": s.engine.Gain()})
}

func (s *Server) handleSetGain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Gain float64 `json:"gain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Gain < 0 || req.Gain > 5.0 {
		writeError(w, http.StatusBadRequest, "gain must be between 0.0 and 5.0")
		return
	}
	s.engine.SetGain(req.Gain)
	s.engine.Broadcast(engine.Event{Type: "gain_changed", Data: map[string]float64{"gain": req.Gain}})
	writeJSON(w, http.StatusOK, map[string]float64{"gain": req.Gain})
}

func (s *Server) handleRecordStart(w http.ResponseWriter, r *http.Request) {
	if err := s.engine.StartRecording(); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	_, startedAt := s.engine.RecordingStatus()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "recording",
		"started_at": startedAt.Format(time.RFC3339),
	})
}

func (s *Server) handleRecordStop(w http.ResponseWriter, r *http.Request) {
	midPath, err := s.engine.StopRecording()
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "rendering",
		"mid":    midPath,
	})
}

func (s *Server) handleRecordStatus(w http.ResponseWriter, r *http.Request) {
	status, startedAt := s.engine.RecordingStatus()
	resp := map[string]interface{}{"status": status}
	if !startedAt.IsZero() {
		resp["started_at"] = startedAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}
