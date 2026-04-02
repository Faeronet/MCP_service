package modules

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	pool *pgxpool.Pool
	cfg  Config
	bot  *BotClient
}

func NewServer(pool *pgxpool.Pool, cfg Config, bot *BotClient) *Server {
	return &Server{pool: pool, cfg: cfg, bot: bot}
}

func (s *Server) Listen(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/schedule/from-note", s.cors(s.handleFromNote))
	mux.HandleFunc("/schedule/list", s.cors(s.handleList))
	return http.ListenAndServe(addr, mux)
}

func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	username := normalizeTelegramUsername(r.URL.Query().Get("telegram_username"))
	if username == "" {
		http.Error(w, "telegram_username required", http.StatusBadRequest)
		return
	}
	tgID, err := lookupUserTelegramID(r.Context(), s.pool, username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT note_item_id
		FROM chat.scheduler_notifications
		WHERE telegram_id = $1
		  AND note_item_id IS NOT NULL
		  AND note_item_id <> ''
		  AND status <> 'cancelled'
	`, tgID)
	if err != nil {
		http.Error(w, "db", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	ids := make([]string, 0, 256)
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && id != "" {
			ids = append(ids, id)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"telegram_username": username, "note_item_ids": ids})
}

func (s *Server) handleFromNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	var req FromNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	res := ProcessFromNote(r.Context(), s.pool, s.bot, req)
	w.Header().Set("Content-Type", "application/json")
	if !res.Accepted {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("encode response: %v", err)
	}
}
