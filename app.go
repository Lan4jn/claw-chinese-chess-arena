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

type arenaRouteHandler func(http.ResponseWriter, *http.Request, string)

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

	arenaRoutes := map[string]map[string]arenaRouteHandler{
		"": {
			http.MethodGet: func(w http.ResponseWriter, r *http.Request, code string) {
				room, err := a.arena.PublicRoom(code)
				if err != nil {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, room)
			},
		},
		"host": {
			http.MethodGet: func(w http.ResponseWriter, r *http.Request, code string) {
				view, err := a.arena.HostRoom(code, r.URL.Query().Get("token"))
				if err != nil {
					writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			},
		},
		"host/match": {
			http.MethodGet: func(w http.ResponseWriter, r *http.Request, code string) {
				view, err := a.arena.HostMatch(code, r.URL.Query().Get("token"))
				if err != nil {
					writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, view)
			},
		},
		"match": {
			http.MethodGet: func(w http.ResponseWriter, r *http.Request, code string) {
				match, err := a.arena.PublicMatch(code)
				if err != nil {
					writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, http.StatusOK, match)
			},
		},
		"match/start": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"match/pause": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"match/resume": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"match/reset": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"move": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"settings": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"seats/assign": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"seats/remove": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"reveal": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"agent/register": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
		"agent/act": {
			http.MethodPost: func(w http.ResponseWriter, r *http.Request, code string) {
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
			},
		},
	}

	mux.HandleFunc("/api/arena/", func(w http.ResponseWriter, r *http.Request) {
		code, tail, ok := arenaRouteParts(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}

		routesByMethod, ok := arenaRoutes[tail]
		if !ok {
			http.NotFound(w, r)
			return
		}

		method := methodForDispatch(r.Method)
		handler, ok := routesByMethod[method]
		if !ok {
			allowedMethods := arenaRouteAllowMethods(routesByMethod)
			methodNotAllowed(w, allowedMethods...)
			return
		}
		handler(w, r, code)
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

func methodForDispatch(method string) string {
	if method == http.MethodHead {
		return http.MethodGet
	}
	return method
}

func arenaRouteAllowMethods(routesByMethod map[string]arenaRouteHandler) []string {
	allowed := make([]string, 0, 2)
	if _, ok := routesByMethod[http.MethodGet]; ok {
		allowed = append(allowed, http.MethodGet, http.MethodHead)
	}
	if _, ok := routesByMethod[http.MethodPost]; ok {
		allowed = append(allowed, http.MethodPost)
	}
	if _, ok := routesByMethod[http.MethodPut]; ok {
		allowed = append(allowed, http.MethodPut)
	}
	if _, ok := routesByMethod[http.MethodPatch]; ok {
		allowed = append(allowed, http.MethodPatch)
	}
	if _, ok := routesByMethod[http.MethodDelete]; ok {
		allowed = append(allowed, http.MethodDelete)
	}
	return allowed
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
