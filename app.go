package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

type App struct {
	arena *Arena
}

func NewApp(store SnapshotStore) *App {
	return &App{arena: NewArena(store)}
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("POST /api/arena/enter", func(w http.ResponseWriter, r *http.Request) {
		var req EnterRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.Enter(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("GET /api/arena/{code}", func(w http.ResponseWriter, r *http.Request) {
		room, err := a.arena.PublicRoom(r.PathValue("code"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, room)
	})

	mux.HandleFunc("GET /api/arena/{code}/host", func(w http.ResponseWriter, r *http.Request) {
		view, err := a.arena.HostRoom(r.PathValue("code"), r.URL.Query().Get("token"))
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("GET /api/arena/{code}/host/match", func(w http.ResponseWriter, r *http.Request) {
		view, err := a.arena.HostMatch(r.PathValue("code"), r.URL.Query().Get("token"))
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("GET /api/arena/{code}/match", func(w http.ResponseWriter, r *http.Request) {
		match, err := a.arena.PublicMatch(r.PathValue("code"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, match)
	})

	mux.HandleFunc("POST /api/arena/{code}/match/start", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HostToken string `json:"host_token"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.StartMatch(r.PathValue("code"), req.HostToken)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("POST /api/arena/{code}/match/pause", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HostToken string `json:"host_token"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.PauseMatch(r.PathValue("code"), req.HostToken)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("POST /api/arena/{code}/match/resume", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HostToken string `json:"host_token"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.ResumeMatch(r.PathValue("code"), req.HostToken)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("POST /api/arena/{code}/match/reset", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HostToken string `json:"host_token"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.ResetMatch(r.PathValue("code"), req.HostToken)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("POST /api/arena/{code}/move", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientToken string `json:"client_token"`
			Move        string `json:"move"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.SubmitMove(r.PathValue("code"), req.ClientToken, req.Move)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("POST /api/arena/{code}/settings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HostToken string `json:"host_token"`
			RoomSettingsRequest
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := a.arena.UpdateSettings(r.PathValue("code"), req.HostToken, req.RoomSettingsRequest); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		room, err := a.arena.HostRoom(r.PathValue("code"), req.HostToken)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, room)
	})

	mux.HandleFunc("POST /api/arena/{code}/seats/assign", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HostToken string `json:"host_token"`
			SeatAssignRequest
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := a.arena.AssignSeat(r.PathValue("code"), req.HostToken, req.SeatAssignRequest); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		room, err := a.arena.HostRoom(r.PathValue("code"), req.HostToken)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, room)
	})

	mux.HandleFunc("POST /api/arena/{code}/seats/remove", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HostToken string   `json:"host_token"`
			Seat      SeatType `json:"seat"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := a.arena.RemoveSeat(r.PathValue("code"), req.HostToken, req.Seat); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		room, err := a.arena.HostRoom(r.PathValue("code"), req.HostToken)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, room)
	})

	mux.HandleFunc("POST /api/arena/{code}/reveal", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HostToken string `json:"host_token"`
			Scope     string `json:"scope"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := a.arena.SetReveal(r.PathValue("code"), req.HostToken, req.Scope); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		room, err := a.arena.HostRoom(r.PathValue("code"), req.HostToken)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, room)
	})

	mux.HandleFunc("POST /api/arena/{code}/agent/register", func(w http.ResponseWriter, r *http.Request) {
		var req AgentRegisterRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.RegisterAgent(r.PathValue("code"), req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	mux.HandleFunc("POST /api/arena/{code}/agent/act", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientToken string `json:"client_token"`
			Move        string `json:"move"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		view, err := a.arena.SubmitMove(r.PathValue("code"), req.ClientToken, req.Move)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, view)
	})

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "index.html"))
	})

	return loggingMiddleware(mux)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("writeJSON failed: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
