package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
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
	Pool        *pgxpool.Pool
	MinIO       *storage.Client
	Queue       *queue.Client
	JWTSecret   []byte
	AdminUser   string
	AdminPass   string
	MCPWriteURL string
	LokiURL     string
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
	claims := jwt.MapClaims{"sub": user, "exp": time.Now().Add(24 * time.Hour).Unix()}
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

	var existingID uuid.UUID
	err = h.Pool.QueryRow(ctx, `SELECT id FROM core.uploads WHERE file_hash = $1`, hash).Scan(&existingID)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "duplicate", "upload_id": existingID.String()})
		return
	}

	docName := r.FormValue("name")
	if docName == "" {
		docName = "document"
	}
	objectKey := "uploads/" + hash
	size, _ := io.Copy(io.Discard, file)
	file.Seek(0, 0)
	_, err = h.MinIO.Put(ctx, objectKey, file, "application/octet-stream", size)
	if err != nil {
		http.Error(w, `{"error":"minio"}`, http.StatusInternalServerError)
		return
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
	_, err = h.Pool.Exec(ctx, `INSERT INTO core.uploads (file_hash, file_path, size_bytes) VALUES ($1, $2, $3) ON CONFLICT (file_hash) DO NOTHING`, hash, objectKey, size)
	if err != nil {
		// continue
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
		docs = append(docs, map[string]interface{}{
			"id": id.String(), "name": name, "created_at": createdAt,
			"versions": rawJSON(versions),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"docs": docs})
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
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if i, _ := strconv.Atoi(l); i > 0 && i <= 500 {
			limit = i
		}
	}
	q := `SELECT ts, level, service, request_id, message, log_id FROM obs.logs_index WHERE 1=1`
	args := []interface{}{}
	n := 0
	if service != "" {
		n++
		q += ` AND service = $` + strconv.Itoa(n)
		args = append(args, service)
	}
	if requestID != "" {
		n++
		q += ` AND request_id = $` + strconv.Itoa(n)
		args = append(args, requestID)
	}
	if level != "" {
		n++
		q += ` AND level = $` + strconv.Itoa(n)
		args = append(args, level)
	}
	n++
	q += ` ORDER BY ts DESC LIMIT $` + strconv.Itoa(n)
	args = append(args, limit)

	rows, err := h.Pool.Query(ctx, q, args...)
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
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"logs": entries})
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

	systemMap := map[string]interface{}{
		"cpu_pct":   system.CPUPct,
		"ram_pct":   system.RAMPct,
		"disk_io_k": system.DiskIOK,
	}
	gpusMap := make([]map[string]interface{}, len(gpus))
	for i, g := range gpus {
		gpusMap[i] = map[string]interface{}{
			"name":     g.Name,
			"gpu_pct":  g.GPUPct,
			"vram_pct": g.VRAMPct,
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

	payload := map[string]interface{}{"system": systemMap, "gpus": gpusMap, "history": historyMap}
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
		u.Path = r.URL.Path
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
			body = grafanaRewriteStaticPaths(body)
		}
		w.Write(body)
	})
}

// grafanaRewriteStaticPaths fixes asset paths when Grafana root_url has no subpath (e.g. localhost:3001).
// So the browser requests /api/grafana/public/... instead of /public/...
func grafanaRewriteStaticPaths(b []byte) []byte {
	s := string(b)
	// Paths that must be under our proxy prefix so the backend receives them
	replacements := []string{
		`"/public/`, `"` + grafanaProxyPrefix + `/public/`,
		`'/public/`, `'` + grafanaProxyPrefix + `/public/`,
		`"/img/`, `"` + grafanaProxyPrefix + `/img/`,
		`'/img/`, `'` + grafanaProxyPrefix + `/img/`,
		`href="/public/`, `href="` + grafanaProxyPrefix + `/public/`,
		`href='/public/`, `href='` + grafanaProxyPrefix + `/public/`,
		`src="/public/`, `src="` + grafanaProxyPrefix + `/public/`,
		`src='/public/`, `src='` + grafanaProxyPrefix + `/public/`,
	}
	for i := 0; i < len(replacements); i += 2 {
		s = strings.ReplaceAll(s, replacements[i], replacements[i+1])
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
