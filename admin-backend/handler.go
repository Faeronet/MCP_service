package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	Pool            *pgxpool.Pool
	MinIO           *storage.Client
	Queue           *queue.Client
	JWTSecret       []byte
	JWTExpiration   time.Duration // срок действия токена (например 168h = 7 дней)
	AdminUser       string
	AdminPass       string
	MCPWriteURL     string
	LokiURL         string
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
	Token string `json:"token"`
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
	claims := jwt.MapClaims{"sub": user, "exp": time.Now().Add(ttl).Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokStr, err := token.SignedString(h.JWTSecret)
	if err != nil {
		http.Error(w, `{"error":"token"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LoginResponse{Token: tokStr})
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
	q := `SELECT role, content, created_at FROM chat.messages WHERE session_id = $1 ORDER BY created_at ASC`
	rows, err := h.Pool.Query(ctx, q, sessionID)
	if err != nil {
		http.Error(w, `{"error":"query"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var messages []map[string]interface{}
	var lastUserAt *time.Time
	for rows.Next() {
		var role, content string
		var createdAt time.Time
		if err := rows.Scan(&role, &content, &createdAt); err != nil {
			continue
		}
		msg := map[string]interface{}{
			"role":       role,
			"content":   content,
			"created_at": createdAt.Format(time.RFC3339),
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
		// Передаём полный путь /api/grafana/... в Grafana (serve_from_sub_path=true)
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
		ct := resp.Header.Get("Content-Type")
		if (strings.Contains(ct, "text/html") || strings.Contains(ct, "javascript")) && len(body) > 0 {
			body = grafanaRewriteStaticPaths(body, r)
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
