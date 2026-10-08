package transport

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/Abhinav-Rao24/Zeplin/memory"
	"github.com/livekit/protocol/auth"
)

// HTTPServerConfig holds configuration for the client HTTP and token server.
type HTTPServerConfig struct {
	Port             string
	LivekitURL       string
	LivekitAPIKey    string
	LivekitAPISecret string
	DefaultRoom      string
	Store            *memory.SQLiteStore
}

// ServeClient starts the embedded HTTP server, serving static frontend assets
// and providing the /api/token and /api/summary endpoints.
//
// Access the client at http://localhost:<port>/
func ServeClient(cfg HTTPServerConfig, clientFS fs.FS) {
	mux := http.NewServeMux()

	// 1. Static client assets
	fileServer := http.FileServer(http.FS(clientFS))
	mux.Handle("/", fileServer)

	// 2. Token minting endpoint (/api/token)
	mux.HandleFunc("/api/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		identity := r.URL.Query().Get("identity")
		if identity == "" {
			identity = "student-1"
		}
		room := r.URL.Query().Get("room")
		if room == "" {
			room = cfg.DefaultRoom
			if room == "" {
				room = "voice-agent-room"
			}
		}

		at := auth.NewAccessToken(cfg.LivekitAPIKey, cfg.LivekitAPISecret)
		grant := &auth.VideoGrant{
			RoomJoin: true,
			Room:     room,
		}
		at.AddGrant(grant).
			SetIdentity(identity).
			SetValidFor(24 * time.Hour)

		token, err := at.ToJWT()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate token: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":    token,
			"url":      cfg.LivekitURL,
			"room":     room,
			"identity": identity,
		})
	})

	// 3. Student practice summary & analytics endpoint (/api/summary)
	if cfg.Store != nil {
		mux.HandleFunc("/api/summary", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			studentID := r.URL.Query().Get("student")
			if studentID == "" {
				studentID = "student-1"
			}
			summary, err := cfg.Store.GetStudentSummary(studentID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get student summary: %v", err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(summary)
		})
	}

	addr := ":" + cfg.Port
	log.Printf("[HTTP] Guitar co-pilot client and API available at http://localhost%s", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("[HTTP] Server error: %v", err)
		}
	}()
}
