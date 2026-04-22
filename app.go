package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
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

	mux.Handle("/api/health", getOrHeadHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})))

	mux.HandleFunc("/api/arena/enter", methodHandler(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
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
	}))

	mux.HandleFunc("/api/arena/", func(w http.ResponseWriter, r *http.Request) {
		code, tail, ok := arenaRouteParts(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}

		allowedMethods := arenaAllowedMethods(tail)
		if len(allowedMethods) == 0 {
			http.NotFound(w, r)
			return
		}
		if !methodAllowed(r.Method, allowedMethods) {
			methodNotAllowed(w, allowedMethods...)
			return
		}

		switch methodForDispatch(r.Method) {
		case http.MethodGet:
			switch tail {
			case "":
				room, err := a.arena.PublicRoom(code)
				if err != nil {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, room)
			case "host":
				view, err := a.arena.HostRoom(code, r.URL.Query().Get("token"))
				if err != nil {
					writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			case "host/match":
				view, err := a.arena.HostMatch(code, r.URL.Query().Get("token"))
				if err != nil {
					writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			case "match":
				match, err := a.arena.PublicMatch(code)
				if err != nil {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, match)
			default:
				http.NotFound(w, r)
			}
		case http.MethodPost:
			switch tail {
			case "match/start":
				var req struct {
					HostToken string `json:"host_token"`
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				view, err := a.arena.StartMatch(code, req.HostToken)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			case "match/pause":
				var req struct {
					HostToken string `json:"host_token"`
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				view, err := a.arena.PauseMatch(code, req.HostToken)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			case "match/resume":
				var req struct {
					HostToken string `json:"host_token"`
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				view, err := a.arena.ResumeMatch(code, req.HostToken)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			case "match/reset":
				var req struct {
					HostToken string `json:"host_token"`
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				view, err := a.arena.ResetMatch(code, req.HostToken)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			case "move":
				var req struct {
					ClientToken string `json:"client_token"`
					Move        string `json:"move"`
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				view, err := a.arena.SubmitMove(code, req.ClientToken, req.Move)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			case "settings":
				var req struct {
					HostToken string `json:"host_token"`
					RoomSettingsRequest
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				if err := a.arena.UpdateSettings(code, req.HostToken, req.RoomSettingsRequest); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				room, err := a.arena.HostRoom(code, req.HostToken)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, room)
			case "seats/assign":
				var req struct {
					HostToken string `json:"host_token"`
					SeatAssignRequest
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				if err := a.arena.AssignSeat(code, req.HostToken, req.SeatAssignRequest); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				room, err := a.arena.HostRoom(code, req.HostToken)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, room)
			case "seats/remove":
				var req struct {
					HostToken string   `json:"host_token"`
					Seat      SeatType `json:"seat"`
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				if err := a.arena.RemoveSeat(code, req.HostToken, req.Seat); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				room, err := a.arena.HostRoom(code, req.HostToken)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, room)
			case "reveal":
				var req struct {
					HostToken string `json:"host_token"`
					Scope     string `json:"scope"`
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				if err := a.arena.SetReveal(code, req.HostToken, req.Scope); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				room, err := a.arena.HostRoom(code, req.HostToken)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, room)
			case "agent/register":
				var req AgentRegisterRequest
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				view, err := a.arena.RegisterAgent(code, req)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			case "agent/act":
				var req struct {
					ClientToken string `json:"client_token"`
					Move        string `json:"move"`
				}
				if err := decodeJSON(r, &req); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				view, err := a.arena.SubmitMove(code, req.ClientToken, req.Move)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			default:
				http.NotFound(w, r)
			}
		}
	})

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", getOrHeadHandler(http.StripPrefix("/static/", fileServer)))
	mux.Handle("/", getOrHeadHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "index.html"))
	})))

	return loggingMiddleware(mux)
}

func methodHandler(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			methodNotAllowed(w, method)
			return
		}
		next(w, r)
	}
}

func getOrHeadHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			methodNotAllowed(w, http.MethodGet, http.MethodHead)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func methodAllowed(method string, allowed []string) bool {
	for _, m := range allowed {
		if method == m {
			return true
		}
	}
	return false
}

func methodForDispatch(method string) string {
	if method == http.MethodHead {
		return http.MethodGet
	}
	return method
}

func arenaAllowedMethods(tail string) []string {
	switch tail {
	case "", "host", "host/match", "match":
		return []string{http.MethodGet, http.MethodHead}
	case "match/start", "match/pause", "match/resume", "match/reset",
		"move", "settings", "seats/assign", "seats/remove", "reveal", "agent/register", "agent/act":
		return []string{http.MethodPost}
	default:
		return nil
	}
}

func arenaRouteParts(path string) (string, string, bool) {
	const prefix = "/api/arena/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", false
	}
	for _, part := range parts {
		if part == "" {
			return "", "", false
		}
	}
	if len(parts) == 1 {
		return parts[0], "", true
	}
	return parts[0], strings.Join(parts[1:], "/"), true
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
