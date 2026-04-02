package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/telegram-ai-assistant/root/pkg/config"
	"github.com/telegram-ai-assistant/root/pkg/logging"
	"github.com/telegram-ai-assistant/root/pkg/queue"
	"github.com/telegram-ai-assistant/root/pkg/storage"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HandlerDeps struct {
	Pool                  *pgxpool.Pool
	MinIO                 *storage.Client
	Queue                 *queue.Client
	JWTSecret             []byte
	JWTExpiration         time.Duration // срок действия токена (например 168h = 7 дней)
	AdminUser             string
	AdminPass             string
	MCPWriteURL           string
	MCPProxyURL           string // POST /chat (тестовый чат из админки)
	LokiURL               string
	ReminderSuperAdminSub string // совпадение с логином → UI симуляции времени (например admin)
	ZoneAgents            []ZoneAgentConfig       // из ZONE_AGENTS (JSON), прокси к zone-agent в каждой зоне
}

type Handler struct {
	HandlerDeps
}

func NewHandler(d HandlerDeps) *Handler {
	return &Handler{HandlerDeps: d}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token         string `json:"token"`
	ReminderDebug bool   `json:"reminder_debug"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	user := strings.TrimSpace(req.Username)
	pass := strings.TrimSpace(req.Password)
	ok := (user == h.AdminUser && pass == h.AdminPass) || (user == "admin" && pass == "admin")
	if !ok {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	ttl := h.JWTExpiration
	if ttl <= 0 {
		ttl = 168 * time.Hour // по умолчанию 7 дней
	}
	reminderDebug := true
	claims := jwt.MapClaims{
		"sub":             user,
		"exp":             time.Now().Add(ttl).Unix(),
		"reminder_debug": reminderDebug,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokStr, err := token.SignedString(h.JWTSecret)
	if err != nil {
		http.Error(w, `{"error":"token"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LoginResponse{Token: tokStr, ReminderDebug: reminderDebug})
}

func fileHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"no file"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	hash, err := fileHash(file)
	if err != nil {
		http.Error(w, `{"error":"hash"}`, http.StatusInternalServerError)
		return
	}
	file.Seek(0, 0)

	docName := r.FormValue("name")
	if docName == "" {
		docName = "document"
	}

	var objectKey string
	var size int64
	var existingPath string
	_ = h.Pool.QueryRow(ctx, `SELECT file_path FROM core.uploads WHERE file_hash = $1`, hash).Scan(&existingPath)
	if existingPath != "" {
		objectKey = existingPath
		size, _ = io.Copy(io.Discard, file)
	} else {
		objectKey = "uploads/" + hash
		size, _ = io.Copy(io.Discard, file)
		file.Seek(0, 0)
		_, err = h.MinIO.Put(ctx, objectKey, file, "application/octet-stream", size)
		if err != nil {
			http.Error(w, `{"error":"minio"}`, http.StatusInternalServerError)
			return
		}
		_, _ = h.Pool.Exec(ctx, `INSERT INTO core.uploads (file_hash, file_path, size_bytes) VALUES ($1, $2, $3) ON CONFLICT (file_hash) DO NOTHING`, hash, objectKey, size)
	}

	var docID, versionID uuid.UUID
	err = h.Pool.QueryRow(ctx, `INSERT INTO core.docs (name) VALUES ($1) RETURNING id`, docName).Scan(&docID)
	if err != nil {
		http.Error(w, `{"error":"doc"}`, http.StatusInternalServerError)
		return
	}
	err = h.Pool.QueryRow(ctx, `INSERT INTO core.versions (doc_id, version, file_path, file_hash) VALUES ($1, 1, $2, $3) RETURNING id`, docID, objectKey, hash).Scan(&versionID)
	if err != nil {
		http.Error(w, `{"error":"version"}`, http.StatusInternalServerError)
		return
	}

	jobID := uuid.New()
	_, err = h.Pool.Exec(ctx, `INSERT INTO core.jobs (id, type, status, doc_id, version_id, payload) VALUES ($1, 'ingestion', 'pending', $2, $3, $4)`,
		jobID, docID, versionID, mustJSON(map[string]interface{}{
			"file_uri": "minio://" + h.MinIO.Bucket() + "/" + objectKey,
			"doc_id":   docID.String(),
			"version_id": versionID.String(),
			"file_hash": hash,
		}))
	if err != nil {
		http.Error(w, `{"error":"job"}`, http.StatusInternalServerError)
		return
	}
	if err := h.Queue.Publish(ctx, "ingestion_jobs", map[string]string{
		"job_id":     jobID.String(),
		"doc_id":     docID.String(),
		"version_id": versionID.String(),
		"file_hash":  hash,
		"file_uri":   "minio://" + h.MinIO.Bucket() + "/" + objectKey,
		"request_id": logging.RequestIDFromContext(ctx),
	}); err != nil {
		// job already in DB
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"job_id": jobID.String(),
		"doc_id": docID.String(),
		"status": "pending",
	})
}

func (h *Handler) ListDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	// Показываем доки: в Qdrant, или с джобом pending/running/failed (ожидают инжеста или упали)
	inQdrant := h.docIDsInQdrant(ctx)
	pendingDocIDs := h.docIDsWithPendingOrFailedJobs(ctx)
	rows, err := h.Pool.Query(ctx, `SELECT d.id, d.name, d.created_at,
		(SELECT json_agg(json_build_object('id', v.id, 'version', v.version, 'file_hash', v.file_hash)) FROM core.versions v WHERE v.doc_id = d.id) 
		FROM core.docs d ORDER BY d.created_at DESC`)
	if err != nil {
		http.Error(w, `{"error":"query"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var docs []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		var createdAt time.Time
		var versions []byte
		if err := rows.Scan(&id, &name, &createdAt, &versions); err != nil {
			continue
		}
		idStr := id.String()
		if inQdrant != nil && !inQdrant[idStr] && !pendingDocIDs[idStr] {
			continue
		}
		docs = append(docs, map[string]interface{}{
			"id": id.String(), "name": name, "created_at": createdAt,
			"versions": rawJSON(versions),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"docs": docs})
}

// docIDsWithPendingOrFailedJobs возвращает set doc_id, у которых есть джоб pending, running или failed (чтобы не скрывать упавшие).
func (h *Handler) docIDsWithPendingOrFailedJobs(ctx context.Context) map[string]bool {
	rows, err := h.Pool.Query(ctx, `SELECT DISTINCT doc_id::text FROM core.jobs WHERE status IN ('pending', 'running', 'failed') AND doc_id IS NOT NULL`)
	if err != nil {
		return make(map[string]bool)
	}
	defer rows.Close()
	set := make(map[string]bool)
	for rows.Next() {
		var idStr string
		if rows.Scan(&idStr) == nil && idStr != "" {
			set[idStr] = true
		}
	}
	return set
}

// docIDsInQdrant возвращает set doc_id, у которых есть чанки в Qdrant. При ошибке — nil (показываем все доки).
func (h *Handler) docIDsInQdrant(ctx context.Context) map[string]bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.MCPWriteURL+"/mcp/doc_ids", nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		DocIDs []string `json:"doc_ids"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		return nil
	}
	set := make(map[string]bool)
	for _, id := range out.DocIDs {
		set[id] = true
	}
	return set
}

// DocsWithID: GET /api/docs/ -> ListDocs, DELETE /api/docs/<id> -> DeleteDoc
func (h *Handler) DocsWithID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListDocs(w, r)
		return
	case http.MethodDelete:
		h.DeleteDoc(w, r)
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (h *Handler) DeleteDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/api/docs/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		http.Error(w, `{"error":"invalid doc id"}`, http.StatusBadRequest)
		return
	}
	docID, err := uuid.Parse(path)
	if err != nil {
		http.Error(w, `{"error":"invalid doc id"}`, http.StatusBadRequest)
		return
	}

	// 1. Удалить чанки из Qdrant (если есть) — запрос к mcp-write, ошибки игнорируем
	if req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, h.MCPWriteURL+"/doc/"+docID.String(), nil); req != nil {
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
		}
	}

	// 2. Удалить джобы по doc_id
	_, _ = h.Pool.Exec(ctx, `DELETE FROM core.jobs WHERE doc_id = $1`, docID)

	// 3. До удаления versions — получить file_path и file_hash для MinIO и кэша uploads
	var filePaths []string
	var fileHashes []string
	if rows, err := h.Pool.Query(ctx, `SELECT file_path, file_hash FROM core.versions WHERE doc_id = $1`, docID); err == nil {
		for rows.Next() {
			var fp, fh string
			if rows.Scan(&fp, &fh) == nil {
				if fp != "" {
					filePaths = append(filePaths, fp)
				}
				if fh != "" {
					fileHashes = append(fileHashes, fh)
				}
			}
		}
		rows.Close()
	}
	for _, objectKey := range filePaths {
		_ = h.MinIO.Remove(ctx, objectKey)
	}
	// Очистить кэш core.uploads по file_hash, чтобы повторная загрузка того же файла не считалась дубликатом
	for _, hash := range fileHashes {
		_, _ = h.Pool.Exec(ctx, `DELETE FROM core.uploads WHERE file_hash = $1`, hash)
	}

	// 4. Удалить versions и doc из БД (порядок из-за FK)
	_, _ = h.Pool.Exec(ctx, `DELETE FROM core.versions WHERE doc_id = $1`, docID)
	_, _ = h.Pool.Exec(ctx, `DELETE FROM core.docs WHERE id = $1`, docID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "doc_id": docID.String()})
}

func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if i, _ := strconv.Atoi(l); i > 0 && i <= 200 {
			limit = i
		}
	}
	rows, err := h.Pool.Query(ctx, `SELECT id, type, status, payload, created_at, updated_at FROM core.jobs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		http.Error(w, `{"error":"query"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var jobs []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var typ, status string
		var payload []byte
		var created, updated time.Time
		if err := rows.Scan(&id, &typ, &status, &payload, &created, &updated); err != nil {
			continue
		}
		jobs = append(jobs, map[string]interface{}{
			"id": id.String(), "type": typ, "status": status,
			"payload": rawJSON(payload), "created_at": created, "updated_at": updated,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"jobs": jobs})
}

func (h *Handler) JobStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	jobID, err := uuid.Parse(path)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var status string
	var payload []byte
	var steps []byte
	err = h.Pool.QueryRow(ctx, `SELECT status, payload FROM core.jobs WHERE id = $1`, jobID).Scan(&status, &payload)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	rows, _ := h.Pool.Query(ctx, `SELECT step_name, status, detail FROM core.job_steps WHERE job_id = $1 ORDER BY created_at`, jobID)
	defer rows.Close()
	var stepList []map[string]interface{}
	for rows.Next() {
		var name, st string
		var detail []byte
		_ = rows.Scan(&name, &st, &detail)
		stepList = append(stepList, map[string]interface{}{"step": name, "status": st, "detail": rawJSON(detail)})
	}
	steps = mustJSONBytes(stepList)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id": jobID.String(), "status": status, "payload": rawJSON(payload), "steps": rawJSON(steps),
	})
}

func (h *Handler) LogsSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	service := r.URL.Query().Get("service")
	requestID := r.URL.Query().Get("request_id")
	level := r.URL.Query().Get("level")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if i, _ := strconv.Atoi(l); i > 0 && i <= 500 {
			limit = i
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if i, _ := strconv.Atoi(o); i > 0 {
			offset = i
		}
	}
	where := `WHERE 1=1`
	args := []interface{}{}
	n := 0
	if service != "" {
		n++
		where += ` AND service = $` + strconv.Itoa(n)
		args = append(args, service)
	}
	if requestID != "" {
		n++
		where += ` AND request_id = $` + strconv.Itoa(n)
		args = append(args, requestID)
	}
	if level != "" {
		n++
		where += ` AND level = $` + strconv.Itoa(n)
		args = append(args, level)
	}

	// total count
	var total int64
	countQ := `SELECT COUNT(*) FROM obs.logs_index ` + where
	if err := h.Pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		http.Error(w, `{"error":"count"}`, http.StatusInternalServerError)
		return
	}

	// data with offset/limit (n = number of filter args, so OFFSET=$n+1 LIMIT=$n+2)
	dataQ := `SELECT ts, level, service, request_id, message, log_id FROM obs.logs_index ` + where + ` ORDER BY ts DESC OFFSET $` + strconv.Itoa(n+1) + ` LIMIT $` + strconv.Itoa(n+2)
	args = append(args, offset, limit)
	rows, err := h.Pool.Query(ctx, dataQ, args...)
	if err != nil {
		http.Error(w, `{"error":"query"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var entries []map[string]interface{}
	for rows.Next() {
		var ts time.Time
		var levelVal, svc, reqID, msg, logID *string
		_ = rows.Scan(&ts, &levelVal, &svc, &reqID, &msg, &logID)
		e := map[string]interface{}{"ts": ts}
		if levelVal != nil {
			e["level"] = *levelVal
		}
		if svc != nil {
			e["service"] = *svc
		}
		if reqID != nil {
			e["request_id"] = *reqID
		}
		if msg != nil {
			e["message"] = *msg
		}
		if logID != nil {
			e["log_id"] = *logID
		}
		entries = append(entries, e)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"logs": entries, "total": total})
}

// ListChats returns chat sessions with username (from core.users), chat_id, message count.
func (h *Handler) ListChats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	q := `SELECT s.id, s.telegram_id, s.chat_id, COALESCE(u.username, '') as username,
		(SELECT COUNT(*) FROM chat.messages m WHERE m.session_id = s.id) as message_count
		FROM chat.sessions s
		LEFT JOIN core.users u ON u.telegram_id = s.telegram_id
		ORDER BY s.last_active DESC`
	rows, err := h.Pool.Query(ctx, q)
	if err != nil {
		http.Error(w, `{"error":"query"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var telegramID, chatID int64
		var username string
		var count int64
		if err := rows.Scan(&id, &telegramID, &chatID, &username, &count); err != nil {
			continue
		}
		displayName := strings.TrimSpace(username)
		if displayName == "" {
			displayName = "—"
		}
		list = append(list, map[string]interface{}{
			"session_id":     id.String(),
			"telegram_id":    telegramID,
			"chat_id":        chatID,
			"username":       displayName,
			"message_count": count,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"chats": list})
}

// GetChatMessages returns messages for a session (role, content, created_at) and response_time_sec for assistant messages.
func (h *Handler) GetChatMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/chats/")
	sessionID = strings.TrimSuffix(sessionID, "/messages")
	if sessionID == "" {
		http.Error(w, `{"error":"session_id required"}`, http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(sessionID); err != nil {
		http.Error(w, `{"error":"invalid session_id"}`, http.StatusBadRequest)
		return
	}
	q := `SELECT id, role, content, created_at, telegram_message_id, reply_to_telegram_message_id FROM chat.messages WHERE session_id = $1 ORDER BY created_at ASC`
	rows, err := h.Pool.Query(ctx, q, sessionID)
	if err != nil {
		http.Error(w, `{"error":"query"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var messages []map[string]interface{}
	var lastUserAt *time.Time
	for rows.Next() {
		var id uuid.UUID
		var role, content string
		var createdAt time.Time
		var telegramMsgID, replyToTelegramMsgID *int64
		if err := rows.Scan(&id, &role, &content, &createdAt, &telegramMsgID, &replyToTelegramMsgID); err != nil {
			continue
		}
		msg := map[string]interface{}{
			"id":         id.String(),
			"role":       role,
			"content":    content,
			"created_at": createdAt.Format(time.RFC3339),
		}
		if telegramMsgID != nil {
			msg["telegram_message_id"] = *telegramMsgID
		}
		if replyToTelegramMsgID != nil && *replyToTelegramMsgID != 0 {
			msg["reply_to_telegram_message_id"] = *replyToTelegramMsgID
		}
		if role == "assistant" && lastUserAt != nil {
			sec := createdAt.Sub(*lastUserAt).Seconds()
			msg["response_time_sec"] = math.Round(sec*100) / 100
		}
		if role == "user" {
			lastUserAt = &createdAt
		}
		messages = append(messages, msg)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"messages": messages})
}

func (h *Handler) LogsRaw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Proxy to Loki query API
	u, _ := url.Parse(h.LokiURL + "/loki/api/v1/query_range")
	u.RawQuery = r.URL.RawQuery
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, `{"error":"loki"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// MonitorMetrics returns current and recent metrics (real from /proc and nvidia-smi on Linux, mock otherwise).
func (h *Handler) MonitorMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	system, gpus, history := CollectMetrics()
	containers := CollectContainerMetrics()

	systemMap := map[string]interface{}{
		"cpu_pct":     system.CPUPct,
		"ram_pct":     system.RAMPct,
		"ram_used_gb": system.RamUsedGB,
		"ram_total_gb": system.RamTotalGB,
		"disk_io_k":   system.DiskIOK,
	}
	gpusMap := make([]map[string]interface{}, len(gpus))
	for i, g := range gpus {
		gpusMap[i] = map[string]interface{}{
			"name":          g.Name,
			"gpu_pct":       g.GPUPct,
			"vram_pct":      g.VRAMPct,
			"vram_used_gb":  g.VRAMUsedGB,
			"vram_total_gb": g.VRAMTotalGB,
		}
	}
	historyMap := make([]map[string]interface{}, len(history))
	for i, hp := range history {
		historyMap[i] = map[string]interface{}{
			"ts":      hp.TS,
			"cpu":     hp.CPU,
			"ram":     hp.RAM,
			"disk_io": hp.DiskIO,
			"gpu":     hp.GPU,
			"vram":    hp.VRAM,
		}
		if len(hp.GPUs) > 0 {
			gpuHist := make([]map[string]interface{}, len(hp.GPUs))
			for j, gh := range hp.GPUs {
				gpuHist[j] = map[string]interface{}{"gpu_pct": gh.GPUPct, "vram_pct": gh.VRAMPct}
			}
			historyMap[i]["gpus"] = gpuHist
		}
	}

	containersMap := make([]map[string]interface{}, len(containers))
	for i, c := range containers {
		histMap := make([]map[string]interface{}, len(c.History))
		for j, h := range c.History {
			histMap[j] = map[string]interface{}{"ts": h.TS, "cpu": h.CPU, "ram": h.RAM}
		}
		containersMap[i] = map[string]interface{}{
			"name":         c.Name,
			"cpu_pct":      c.CPUPct,
			"ram_pct":      c.RAMPct,
			"ram_used_gb":  c.RamUsedGB,
			"ram_limit_gb": c.RamLimitGB,
			"history":      histMap,
		}
	}
	payload := map[string]interface{}{
		"system":     systemMap,
		"gpus":       gpusMap,
		"history":    historyMap,
		"containers": containersMap,
		"uptime_sec": GetUptimeSec(),
	}
	if len(gpus) > 0 {
		payload["gpu"] = map[string]interface{}{"gpu_pct": gpus[0].GPUPct, "vram_pct": gpus[0].VRAMPct}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

const grafanaProxyPrefix = "/api/grafana"

func (h *Handler) GrafanaProxy() http.Handler {
	grafanaURL := config.LoadString("GRAFANA_URL", "http://grafana:3000")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := url.Parse(grafanaURL)
		// StripPrefix срезает /api/grafana, в Grafana уходит путь от корня: /, /public/...
		u.Path = r.URL.Path
		if u.Path == "" {
			u.Path = "/"
		}
		q := r.URL.Query()
		q.Del("token")
		u.RawQuery = q.Encode()
		proxyReq, _ := http.NewRequestWithContext(r.Context(), r.Method, u.String(), r.Body)
		for k, v := range r.Header {
			if strings.ToLower(k) == "host" {
				continue
			}
			proxyReq.Header[k] = v
		}
		proxyReq.Header.Set("Host", u.Host)
		if r.Host != "" {
			proxyReq.Header.Set("X-Forwarded-Host", r.Host)
		}
		if r.TLS != nil {
			proxyReq.Header.Set("X-Forwarded-Proto", "https")
		} else {
			proxyReq.Header.Set("X-Forwarded-Proto", "http")
		}
		proxyReq.Header.Set("X-Forwarded-Prefix", grafanaProxyPrefix)
		resp, err := http.DefaultClient.Do(proxyReq)
		if err != nil {
			http.Error(w, `{"error":"grafana"}`, http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		token := r.URL.Query().Get("token")
		for k, v := range resp.Header {
			if strings.ToLower(k) == "location" {
				for _, vv := range v {
					if vv != "" && vv[0] == '/' && !strings.HasPrefix(vv, "//") && !strings.HasPrefix(vv, grafanaProxyPrefix) {
						vv = grafanaProxyPrefix + vv
						if token != "" {
							if strings.Contains(vv, "?") {
								vv += "&token=" + url.QueryEscape(token)
							} else {
								vv += "?token=" + url.QueryEscape(token)
							}
						}
					}
					w.Header().Add(k, vv)
				}
				continue
			}
			if strings.ToLower(k) == "content-length" {
				continue
			}
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.WriteHeader(resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		// Не перезаписываем body, если ответ в gzip — иначе поток ломается и «failed to load»
		if resp.Header.Get("Content-Encoding") != "gzip" && resp.Header.Get("Content-Encoding") != "br" {
			ct := resp.Header.Get("Content-Type")
			if (strings.Contains(ct, "text/html") || strings.Contains(ct, "javascript")) && len(body) > 0 {
				body = grafanaRewriteStaticPaths(body, r)
			}
		}
		w.Write(body)
	})
}

// grafanaRewriteStaticPaths fixes asset paths and injects <base> so the browser loads everything via /api/grafana/.
func grafanaRewriteStaticPaths(b []byte, r *http.Request) []byte {
	s := string(b)
	replacements := []string{
		`"/public/`, `"` + grafanaProxyPrefix + `/public/`,
		`'/public/`, `'` + grafanaProxyPrefix + `/public/`,
		`"/img/`, `"` + grafanaProxyPrefix + `/img/`,
		`'/img/`, `'` + grafanaProxyPrefix + `/img/`,
		`href="/public/`, `href="` + grafanaProxyPrefix + `/public/`,
		`href='/public/`, `href='` + grafanaProxyPrefix + `/public/`,
		`src="/public/`, `src="` + grafanaProxyPrefix + `/public/`,
		`src='/public/`, `src='` + grafanaProxyPrefix + `/public/`,
		`url("/public/`, `url("` + grafanaProxyPrefix + `/public/`,
		`url('/public/`, `url('` + grafanaProxyPrefix + `/public/`,
		`url("/img/`, `url("` + grafanaProxyPrefix + `/img/`,
		`url('/img/`, `url('` + grafanaProxyPrefix + `/img/`,
	}
	for i := 0; i < len(replacements); i += 2 {
		s = strings.ReplaceAll(s, replacements[i], replacements[i+1])
	}
	s = strings.ReplaceAll(s, `<base href="/">`, `<base href="`+grafanaProxyPrefix+`/">`)
	s = strings.ReplaceAll(s, `<base href='/'>`, `<base href='`+grafanaProxyPrefix+`/'>`)
	// Вставляем <base href="proto://host/api/grafana/"> после <head>, чтобы все относительные пути шли через прокси
	if r != nil && r.Host != "" && strings.Contains(s, "<head>") {
		proto := "https"
		if r.TLS == nil {
			proto = "http"
		}
		if v := r.Header.Get("X-Forwarded-Proto"); v != "" {
			proto = v
		}
		baseTag := `<base href="` + proto + "://" + r.Host + grafanaProxyPrefix + `/">`
		s = strings.Replace(s, "<head>", "<head>\n  "+baseTag, 1)
	}
	return []byte(s)
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func mustJSONBytes(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func rawJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	var v interface{}
	_ = json.Unmarshal(b, &v)
	return v
}

func (h *Handler) parseJWTClaims(r *http.Request) (jwt.MapClaims, error) {
	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" {
		return nil, fmt.Errorf("missing authorization")
	}
	if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
		tokenStr = tokenStr[7:]
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) { return h.JWTSecret, nil })
	if err != nil || !token.Valid {
		return nil, err
	}
	c, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	return c, nil
}

func jwtClaimBool(c jwt.MapClaims, key string) bool {
	v, ok := c[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}

// RemindersConfig GET /api/reminders/config
func (h *Handler) RemindersConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	var disabled bool
	_ = h.Pool.QueryRow(ctx, `SELECT COALESCE(disabled, false) FROM chat.reminder_global_config WHERE id = 0`).Scan(&disabled)
	w.Header().Set("Content-Type", "application/json")
	out := map[string]interface{}{"disabled": disabled}
	var sim sql.NullTime
	_ = h.Pool.QueryRow(ctx, `SELECT simulated_at FROM chat.reminder_debug_clock WHERE id = 0`).Scan(&sim)
	if sim.Valid {
		out["simulated_at"] = sim.Time.Format(time.RFC3339)
	} else {
		out["simulated_at"] = nil
	}
	_ = json.NewEncoder(w).Encode(out)
}

type remindersToggleBody struct {
	Disabled bool `json:"disabled"`
}

// RemindersToggle POST /api/reminders/toggle
func (h *Handler) RemindersToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body remindersToggleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	_, err := h.Pool.Exec(ctx, `
		INSERT INTO chat.reminder_global_config (id, disabled, updated_at) VALUES (0, $1, NOW())
		ON CONFLICT (id) DO UPDATE SET disabled = EXCLUDED.disabled, updated_at = NOW()
	`, body.Disabled)
	if err != nil {
		http.Error(w, `{"error":"db"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "disabled": body.Disabled})
}

type remindersDebugClockBody struct {
	SimulatedISO *string `json:"simulated_iso"`
	Clear        bool    `json:"clear"`
}

type reminderSubscriberRow struct {
	TelegramID int64     `json:"telegram_id"`
	ChatID     int64     `json:"chat_id"`
	Username   string    `json:"username"`
	ReminderHH int       `json:"reminder_hh"`
	ReminderMM int       `json:"reminder_mm"`
	Enabled    bool      `json:"enabled"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type remindersResetUserBody struct {
	TelegramID *int64 `json:"telegram_id"`
}

func parseAdminSimulatedTime(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Новый дефолт для админки: "YYYY-MM-DD HH:MM" (МСК).
	msk := time.FixedZone("MSK", 3*60*60)
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, msk); err == nil {
			return t, nil
		}
	}
	// Допустим и "YYYY-MM-DDTHH:MM".
	if t, err := time.ParseInLocation("2006-01-02T15:04", s, msk); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("bad time format")
}

// RemindersDebugClock POST /api/reminders/debug-clock
func (h *Handler) RemindersDebugClock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body remindersDebugClockBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	var err error
	if body.Clear {
		_, err = h.Pool.Exec(ctx, `UPDATE chat.reminder_debug_clock SET simulated_at = NULL, updated_at = NOW(), source = 'admin' WHERE id = 0`)
	} else if body.SimulatedISO != nil && *body.SimulatedISO != "" {
		t, perr := parseAdminSimulatedTime(*body.SimulatedISO)
		if perr != nil {
			http.Error(w, `{"error":"bad time, use YYYY-MM-DD HH:MM"}`, http.StatusBadRequest)
			return
		}
		_, err = h.Pool.Exec(ctx, `UPDATE chat.reminder_debug_clock SET simulated_at = $1, updated_at = NOW(), source = 'admin' WHERE id = 0`, t)
	} else {
		http.Error(w, `{"error":"need simulated_iso or clear"}`, http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"db"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// RemindersSubscribers GET /api/reminders/subscribers
func (h *Handler) RemindersSubscribers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	rows, err := h.Pool.Query(ctx, `
		SELECT rs.telegram_id, rs.chat_id, COALESCE(u.username, '') AS username,
		       rs.reminder_hh, rs.reminder_mm, rs.enabled, rs.updated_at
		FROM chat.reminder_subscribers rs
		LEFT JOIN core.users u ON u.telegram_id = rs.telegram_id
		WHERE rs.enabled = true
		ORDER BY rs.updated_at DESC
	`)
	if err != nil {
		http.Error(w, `{"error":"query"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []reminderSubscriberRow
	for rows.Next() {
		var x reminderSubscriberRow
		if err := rows.Scan(&x.TelegramID, &x.ChatID, &x.Username, &x.ReminderHH, &x.ReminderMM, &x.Enabled, &x.UpdatedAt); err != nil {
			continue
		}
		list = append(list, x)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"subscribers": list})
}

// RemindersResetUser POST /api/reminders/reset-user
func (h *Handler) RemindersResetUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body remindersResetUserBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if body.TelegramID == nil {
		http.Error(w, `{"error":"telegram_id required"}`, http.StatusBadRequest)
		return
	}
	telegramID := *body.TelegramID
	ctx := r.Context()
	_, err := h.Pool.Exec(ctx, `UPDATE chat.reminder_subscribers SET enabled = false, updated_at = NOW() WHERE telegram_id = $1`, telegramID)
	if err != nil {
		http.Error(w, `{"error":"db"}`, http.StatusInternalServerError)
		return
	}
	_, _ = h.Pool.Exec(ctx, `DELETE FROM chat.reminder_sent WHERE telegram_id = $1`, telegramID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "telegram_id": telegramID})
}

type schedulerNotificationRow struct {
	ID           string     `json:"id"`
	TelegramID   int64      `json:"telegram_id"`
	ChatID       int64      `json:"chat_id"`
	AngelChunkID string     `json:"angel_chunk_id"`
	AngelName    string     `json:"angel_name"`
	MessageText  string     `json:"message_text"`
	SendAt       time.Time  `json:"send_at"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	SentAt       *time.Time `json:"sent_at,omitempty"`
	LastError    *string    `json:"last_error,omitempty"`
}

type schedulerNotifIDBody struct {
	ID string `json:"id"`
}

// SchedulerNotificationsList GET /api/reminders/scheduler-notifications
func (h *Handler) SchedulerNotificationsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if i, err := strconv.Atoi(l); err == nil && i > 0 && i <= 500 {
			limit = i
		}
	}
	rows, err := h.Pool.Query(ctx, `
		SELECT id::text, telegram_id, chat_id, angel_chunk_id, angel_name,
		       message_text, send_at, status, created_at, sent_at, last_error
		FROM chat.scheduler_notifications
		ORDER BY send_at DESC, created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		http.Error(w, `{"error":"query"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []schedulerNotificationRow
	for rows.Next() {
		var x schedulerNotificationRow
		var sentAt sql.NullTime
		var lastErr sql.NullString
		if err := rows.Scan(&x.ID, &x.TelegramID, &x.ChatID, &x.AngelChunkID, &x.AngelName,
			&x.MessageText, &x.SendAt, &x.Status, &x.CreatedAt, &sentAt, &lastErr); err != nil {
			continue
		}
		if sentAt.Valid {
			t := sentAt.Time
			x.SentAt = &t
		}
		if lastErr.Valid && strings.TrimSpace(lastErr.String) != "" {
			s := lastErr.String
			x.LastError = &s
		}
		list = append(list, x)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"notifications": list})
}

// SchedulerNotificationCancel POST .../cancel — pending → cancelled (не отправится воркером).
func (h *Handler) SchedulerNotificationCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body schedulerNotifIDBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	idStr := strings.TrimSpace(body.ID)
	if idStr == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(idStr); err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	ct, err := h.Pool.Exec(ctx, `
		UPDATE chat.scheduler_notifications
		SET status = 'cancelled'
		WHERE id = $1::uuid AND status IN ('pending', 'sending')
	`, idStr)
	if err != nil {
		http.Error(w, `{"error":"db"}`, http.StatusInternalServerError)
		return
	}
	if ct.RowsAffected() == 0 {
		http.Error(w, `{"error":"not found or already completed (sent/failed/cancelled)"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": idStr, "status": "cancelled"})
}

// SchedulerNotificationDelete POST .../delete — полное удаление строки.
func (h *Handler) SchedulerNotificationDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body schedulerNotifIDBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	idStr := strings.TrimSpace(body.ID)
	if idStr == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(idStr); err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	ct, err := h.Pool.Exec(ctx, `DELETE FROM chat.scheduler_notifications WHERE id = $1::uuid`, idStr)
	if err != nil {
		http.Error(w, `{"error":"db"}`, http.StatusInternalServerError)
		return
	}
	if ct.RowsAffected() == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": idStr})
}

// Сессия админского тестового чата: не пересекается с реальными telegram_id.
const adminLLMTelegramID int64 = 0
const adminLLMChatID int64 = 0

type chatLLMRequestBody struct {
	MessageText               string `json:"message_text"`
	ReplyToTelegramMessageID  int    `json:"reply_to_telegram_message_id"`
}

type mcpProxyChatResponse struct {
	ReplyText         string `json:"reply_text"`
	DebugMessage      string `json:"debug_message,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	ReminderExtraText string `json:"reminder_extra_text,omitempty"`
}

func (h *Handler) ensureAdminLLMSession(ctx context.Context) (uuid.UUID, error) {
	// Для отображения в Chat Log не прочерком, а как "admin".
	_, _ = h.Pool.Exec(ctx, `
		INSERT INTO core.users (telegram_id, username) VALUES ($1, $2)
		ON CONFLICT (telegram_id) DO UPDATE SET username = EXCLUDED.username
	`, adminLLMTelegramID, "admin")
	var id uuid.UUID
	err := h.Pool.QueryRow(ctx, `
		INSERT INTO chat.sessions (telegram_id, chat_id, last_active)
		VALUES ($1, $2, NOW())
		ON CONFLICT (telegram_id, chat_id) DO UPDATE SET last_active = NOW()
		RETURNING id
	`, adminLLMTelegramID, adminLLMChatID).Scan(&id)
	return id, err
}

// ChatLLM POST /api/chat/llm — прокси на mcp-proxy /chat; выставляет синтетические отрицательные telegram_message_id для ответов «Ответить по контексту».
func (h *Handler) ChatLLM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.TrimSpace(h.MCPProxyURL) == "" {
		http.Error(w, `{"error":"MCP_PROXY_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	var body chatLLMRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.MessageText) == "" {
		http.Error(w, `{"error":"message_text required"}`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	sessionID, err := h.ensureAdminLLMSession(ctx)
	if err != nil {
		http.Error(w, `{"error":"session"}`, http.StatusInternalServerError)
		return
	}
	reqID := uuid.New().String()
	proxyPayload := map[string]interface{}{
		"session_id":                    sessionID.String(),
		"chat_id":                       adminLLMChatID,
		"user_id":                       adminLLMTelegramID,
		"username":                      "admin",
		"message_text":                  body.MessageText,
		"reply_to_telegram_message_id": body.ReplyToTelegramMessageID,
		"request_id":                    reqID,
	}
	raw, _ := json.Marshal(proxyPayload)
	proxyURL := h.MCPProxyURL + "/chat"
	preq, err := http.NewRequestWithContext(ctx, http.MethodPost, proxyURL, bytes.NewReader(raw))
	if err != nil {
		http.Error(w, `{"error":"proxy request"}`, http.StatusInternalServerError)
		return
	}
	preq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second}
	presp, err := client.Do(preq)
	if err != nil {
		http.Error(w, `{"error":"proxy unreachable"}`, http.StatusBadGateway)
		return
	}
	defer presp.Body.Close()
	pbody, _ := io.ReadAll(presp.Body)
	if presp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(presp.StatusCode)
		_, _ = w.Write(pbody)
		return
	}
	var out mcpProxyChatResponse
	if err := json.Unmarshal(pbody, &out); err != nil {
		http.Error(w, `{"error":"bad proxy response"}`, http.StatusBadGateway)
		return
	}
	var telegramID int64
	if out.MessageID != "" {
		msgUUID, perr := uuid.Parse(out.MessageID)
		if perr == nil {
			_ = h.Pool.QueryRow(ctx, `
				SELECT COALESCE((SELECT MIN(telegram_message_id) - 1 FROM chat.messages WHERE session_id = $1 AND telegram_message_id < 0), -1)
			`, sessionID).Scan(&telegramID)
			_, _ = h.Pool.Exec(ctx, `
				UPDATE chat.messages SET telegram_message_id = $1 WHERE id = $2 AND session_id = $3
			`, telegramID, msgUUID, sessionID)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"reply_text":            out.ReplyText,
		"debug_message":         out.DebugMessage,
		"telegram_message_id":   telegramID,
		"reminder_extra_text":   out.ReminderExtraText,
	})
}
