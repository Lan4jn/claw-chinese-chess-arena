package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type matchView struct {
	Match      *Match   `json:"match"`
	BoardText  string   `json:"board_text"`
	LegalMoves []string `json:"legal_moves"`
}

type errorResponse struct {
	Error string     `json:"error"`
	Match *matchView `json:"match,omitempty"`
}

func main() {
	addr := normalizeListenAddr(os.Getenv("PORT"))
	manager := NewManager()

	server := &http.Server{
		Addr:              addr,
		Handler:           routes(manager),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Pico Xiangqi Arena listening on http://localhost%s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func routes(manager *Manager) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("GET /api/matches", func(w http.ResponseWriter, r *http.Request) {
		matches := manager.List()
		slices.SortFunc(matches, func(a, b *Match) int {
			return b.UpdatedAt.Compare(a.UpdatedAt)
		})
		writeJSON(w, http.StatusOK, matches)
	})

	mux.HandleFunc("POST /api/matches", func(w http.ResponseWriter, r *http.Request) {
		var req CreateMatchRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		match, err := manager.Create(req)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, buildMatchView(match))
	})

	mux.HandleFunc("GET /api/matches/{id}", func(w http.ResponseWriter, r *http.Request) {
		match, ok := manager.Get(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "match not found"})
			return
		}
		writeJSON(w, http.StatusOK, buildMatchView(match))
	})

	mux.HandleFunc("GET /api/matches/{id}/legal", func(w http.ResponseWriter, r *http.Request) {
		match, ok := manager.Get(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "match not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string][]string{"legal_moves": legalMoves(match)})
	})

	mux.HandleFunc("POST /api/matches/{id}/move", func(w http.ResponseWriter, r *http.Request) {
		var req ManualMoveRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		match, err := manager.ManualMove(r.PathValue("id"), req.Move)
		if err != nil {
			status := http.StatusBadRequest
			if match == nil {
				status = http.StatusNotFound
			}
			writeJSON(w, status, errorResponse{Error: err.Error(), Match: optionalView(match)})
			return
		}
		writeJSON(w, http.StatusOK, buildMatchView(match))
	})

	mux.HandleFunc("POST /api/matches/{id}/step", func(w http.ResponseWriter, r *http.Request) {
		match, err := manager.Step(r.Context(), r.PathValue("id"))
		if err != nil {
			status := http.StatusBadRequest
			if match == nil {
				status = http.StatusNotFound
			}
			writeJSON(w, status, errorResponse{Error: err.Error(), Match: optionalView(match)})
			return
		}
		writeJSON(w, http.StatusOK, buildMatchView(match))
	})

	mux.HandleFunc("POST /api/matches/{id}/auto", func(w http.ResponseWriter, r *http.Request) {
		var req AutoRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		match, err := manager.SetAuto(r.PathValue("id"), req)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, buildMatchView(match))
	})

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "index.html"))
	})

	return loggingMiddleware(mux)
}

func buildMatchView(match *Match) matchView {
	return matchView{
		Match:      match,
		BoardText:  BoardText(match.State.Board),
		LegalMoves: legalMoves(match),
	}
}

func optionalView(match *Match) *matchView {
	if match == nil {
		return nil
	}
	view := buildMatchView(match)
	return &view
}

func legalMoves(match *Match) []string {
	if match == nil || match.State.Status != "playing" {
		return nil
	}
	return match.State.LegalMoveStrings()
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

func normalizeListenAddr(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return ":8080"
	}
	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
