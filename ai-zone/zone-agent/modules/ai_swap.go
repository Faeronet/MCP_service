package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type aiSwapServiceDef struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	ComposeSvc    string `json:"compose_service"`
	EnvKey        string `json:"env_key"`
	CurrentModel  string `json:"current_model"`
	ModelsDir     string `json:"models_dir"`
	UsesHostCache bool   `json:"uses_host_hf_cache"`
}

type aiSwapStatusResponse struct {
	Services           []aiSwapServiceDef `json:"services"`
	AIStoreConfigured  bool               `json:"ai_store_configured"`
	AIStoreHint        string             `json:"ai_store_hint,omitempty"`
	CatalogFile        string             `json:"catalog_file,omitempty"`
}

type catalogModel struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type aiSwapCatalogResponse struct {
	Service string         `json:"service"`
	Models  []catalogModel `json:"models"`
	Stub    bool           `json:"stub"`
}

type aiSwapSwapRequest struct {
	Service string `json:"service"`
	ModelID string `json:"model_id"`
}

var aiSwapRegistry = map[string]struct {
	Label   string
	EnvKey  string
	Default string
	HFCache bool
}{
	"vllm":       {"LLM", "VLLM_MODEL_NAME", "Qwen/Qwen3-14B-AWQ", true},
	"vllm-embed": {"Embedding", "VLLM_EMBED_MODEL", "BAAI/bge-m3", true},
	"rerank":     {"Rerank", "RERANK_MODEL", "BAAI/bge-reranker-v2-m3", false},
}

func (s *server) aiStoreConfigured() bool {
	endpoint := strings.TrimSpace(os.Getenv("AI_STORE_ENDPOINT"))
	bucket := strings.TrimSpace(os.Getenv("AI_STORE_BUCKET"))
	return endpoint != "" && bucket != ""
}

func hfHubModelDirName(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ""
	}
	parts := strings.Split(modelID, "/")
	var b strings.Builder
	b.WriteString("models--")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i > 0 {
			b.WriteString("--")
		}
		var sb strings.Builder
		for _, r := range p {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
				sb.WriteRune(r)
			default:
				sb.WriteRune('-')
			}
		}
		b.WriteString(sb.String())
	}
	return b.String()
}

func parseDotEnv(path string) map[string]string {
	out := make(map[string]string)
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func quoteEnvValue(val string) string {
	if !strings.ContainsAny(val, " \t\n\"'\\#") {
		return val
	}
	esc := strings.ReplaceAll(val, `\`, `\\`)
	esc = strings.ReplaceAll(esc, `"`, `\"`)
	return `"` + esc + `"`
}

func writeDotEnvKey(path, key, value string) error {
	b, err := os.ReadFile(path)
	content := ""
	if err == nil {
		content = string(b)
	} else if os.IsNotExist(err) {
		content = ""
	} else {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	found := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		k, _, ok := strings.Cut(t, "=")
		if ok && strings.TrimSpace(k) == key {
			lines[i] = key + "=" + quoteEnvValue(value)
			found = true
			break
		}
	}
	if !found {
		if len(lines) == 1 && lines[0] == "" {
			lines = lines[:0]
		}
		lines = append(lines, key+"="+quoteEnvValue(value))
	}
	out := strings.Join(lines, "\n")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	tmp := path + ".tmp." + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := os.WriteFile(tmp, []byte(out), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *server) modelsRootDir() string {
	return filepath.Join(s.workdir, "models")
}

func (s *server) removeHFCacheForModel(log func(string), modelID string) {
	dirName := hfHubModelDirName(modelID)
	if dirName == "" {
		return
	}
	full := filepath.Join(s.modelsRootDir(), "hub", dirName)
	log(fmt.Sprintf("[ai-swap] удаление кэша Hugging Face: %s", full))
	if err := os.RemoveAll(full); err != nil {
		log(fmt.Sprintf("[ai-swap] предупреждение: не удалось удалить %s: %v", full, err))
	}
}

func (s *server) pullModelFromStore(ctx context.Context, log func(string), modelID, service string) error {
	_ = ctx
	endpoint := strings.TrimSpace(os.Getenv("AI_STORE_ENDPOINT"))
	bucket := strings.TrimSpace(os.Getenv("AI_STORE_BUCKET"))
	access := strings.TrimSpace(os.Getenv("AI_STORE_ACCESS_KEY"))
	secret := strings.TrimSpace(os.Getenv("AI_STORE_SECRET_KEY"))
	if endpoint == "" || bucket == "" {
		log("[ai-swap] AI Store не настроен (AI_STORE_ENDPOINT, AI_STORE_BUCKET). Загрузка из хранилища пропущена — при старте контейнер может скачать модель из Hugging Face.")
		return nil
	}
	if access == "" || secret == "" {
		log("[ai-swap] AI Store: задайте AI_STORE_ACCESS_KEY и AI_STORE_SECRET_KEY для загрузки (пока не реализовано).")
		return nil
	}
	log(fmt.Sprintf("[ai-swap] (заглушка) скачивание %s для %s из s3://%s @ %s — клиент MinIO/S3 будет добавлен позже.", modelID, service, bucket, endpoint))
	return nil
}

func defaultCatalog() map[string][]catalogModel {
	return map[string][]catalogModel{
		"vllm": {
			{ID: "Qwen/Qwen3-14B-AWQ", Label: "Qwen3 14B AWQ"},
			{ID: "Qwen/Qwen2.5-7B-Instruct", Label: "Qwen2.5 7B Instruct"},
		},
		"vllm-embed": {
			{ID: "BAAI/bge-m3", Label: "BGE-M3"},
			{ID: "intfloat/multilingual-e5-large", Label: "multilingual-e5-large"},
		},
		"rerank": {
			{ID: "BAAI/bge-reranker-v2-m3", Label: "BGE reranker v2-m3"},
		},
	}
}

func (s *server) catalogByService() map[string][]catalogModel {
	path := filepath.Join(s.workdir, "ai-swap-catalog.json")
	raw, err := os.ReadFile(path)
	if err == nil {
		var m map[string][]catalogModel
		if json.Unmarshal(raw, &m) == nil && len(m) > 0 {
			return m
		}
	}
	return defaultCatalog()
}

func (s *server) currentModelsFromEnv() map[string]string {
	env := parseDotEnv(s.envPath())
	out := make(map[string]string)
	for svc, def := range aiSwapRegistry {
		if v := strings.TrimSpace(env[def.EnvKey]); v != "" {
			out[svc] = v
		} else {
			out[svc] = def.Default
		}
	}
	return out
}

func (s *server) handleAISwapStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	cur := s.currentModelsFromEnv()
	modelsDir := s.modelsRootDir()
	svcs := make([]aiSwapServiceDef, 0, len(aiSwapRegistry))
	order := []string{"vllm", "vllm-embed", "rerank"}
	for _, id := range order {
		def := aiSwapRegistry[id]
		svcs = append(svcs, aiSwapServiceDef{
			ID:            id,
			Label:         def.Label,
			ComposeSvc:    id,
			EnvKey:        def.EnvKey,
			CurrentModel:  cur[id],
			ModelsDir:     modelsDir,
			UsesHostCache: def.HFCache,
		})
	}
	resp := aiSwapStatusResponse{
		Services:          svcs,
		AIStoreConfigured: s.aiStoreConfigured(),
		CatalogFile:       filepath.Join(s.workdir, "ai-swap-catalog.json"),
	}
	if !resp.AIStoreConfigured {
		resp.AIStoreHint = "Задайте AI_STORE_ENDPOINT и AI_STORE_BUCKET (и ключи) для загрузки моделей из MinIO; пока используется каталог и пересборка с .env."
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) handleAISwapCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	svc := strings.TrimSpace(r.URL.Query().Get("service"))
	if _, ok := aiSwapRegistry[svc]; !ok {
		http.Error(w, `{"error":"unknown service"}`, http.StatusBadRequest)
		return
	}
	cat := s.catalogByService()
	list := cat[svc]
	if list == nil {
		list = []catalogModel{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(aiSwapCatalogResponse{
		Service: svc,
		Models:  list,
		Stub:    !s.aiStoreConfigured(),
	})
}

func (s *server) handleAISwapSwap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method"}`, http.StatusMethodNotAllowed)
		return
	}
	var req aiSwapSwapRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	req.Service = strings.TrimSpace(req.Service)
	req.ModelID = strings.TrimSpace(req.ModelID)
	def, ok := aiSwapRegistry[req.Service]
	if !ok || req.ModelID == "" {
		http.Error(w, `{"error":"service and model_id required"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	log := func(msg string) {
		_, _ = fmt.Fprintf(w, "%s\n", msg)
		if fl != nil {
			fl.Flush()
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Minute)
	defer cancel()

	cur := s.currentModelsFromEnv()
	oldModel := cur[req.Service]
	log(fmt.Sprintf("[ai-swap] сервис=%s текущая_модель=%s -> %s", req.Service, oldModel, req.ModelID))

	if def.HFCache && oldModel != "" && oldModel != req.ModelID {
		s.removeHFCacheForModel(log, oldModel)
	} else if !def.HFCache {
		log("[ai-swap] rerank: кэш HF на хосте ./models не используется; образ пересоберётся и подтянет веса при старте.")
	}

	if err := s.pullModelFromStore(ctx, log, req.ModelID, req.Service); err != nil {
		log(fmt.Sprintf("[ai-swap] ошибка загрузки из store: %v", err))
		return
	}

	envPath := s.envPath()
	if err := writeDotEnvKey(envPath, def.EnvKey, req.ModelID); err != nil {
		log(fmt.Sprintf("[ai-swap] ошибка записи .env (%s): %v", def.EnvKey, err))
		return
	}
	log(fmt.Sprintf("[ai-swap] .env обновлён: %s=%s", def.EnvKey, req.ModelID))

	composePath, err := s.composeFile()
	if err != nil {
		log("[ai-swap] ошибка: " + err.Error())
		return
	}
	log("=== /etc/resolv.conf (patch для сборки) ===")
	var resolvLog strings.Builder
	s.patchHostResolvForBuild(ctx, &resolvLog)
	log(strings.TrimSpace(resolvLog.String()))

	base := append([]string{"docker"}, s.dockerComposeBaseArgs(composePath)...)
	log(fmt.Sprintf("=== docker compose up -d --build --force-recreate %s ===", req.Service))
	out, runErr := runCmd(ctx, base[0], composeUpBuildArgs(base[1:], req.Service)...)
	log(string(out))
	if runErr != nil {
		log(runErr.Error())
		log("[ai-swap] готово с ошибкой пересборки.")
		return
	}
	log("[ai-swap] готово.")
}
