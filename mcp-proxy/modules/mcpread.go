package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

)

// BuildContext calls mcp-read /mcp/build_context; returns context, chunkIDs, collection, etc.
func (s *Server) BuildContext(ctx context.Context, requestID, query, attachmentsText string, tokenBudget int, mode string) (context string, chunkIDs []string, searchCollection string, collectionsSearched []string, queryForFilter string, contextKind string, contextRef string, err error) {
	body := map[string]interface{}{
		"query_text":       query,
		"acl_token":        "placeholder",
		"token_budget":     tokenBudget,
		"mode":             mode,
		"attachments_text": attachmentsText,
	}
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.McpReadURL+"/mcp/build_context", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, "", nil, "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		return "", nil, "", nil, "", "", "", fmt.Errorf("mcp-read %d: %s", resp.StatusCode, string(bb))
	}
	var out struct {
		Context             string   `json:"context"`
		ChunkIDs            []string `json:"chunk_ids"`
		SearchCollection    string   `json:"search_collection"`
		CollectionsSearched []string `json:"collections_searched"`
		QueryForFilter      string   `json:"query_for_filter"`
		ContextKind         string   `json:"context_kind"`
		ContextRef          string   `json:"context_ref"`
		Error               string   `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", nil, "", nil, "", "", "", err
	}
	if out.Error != "" {
		return "", out.ChunkIDs, out.SearchCollection, out.CollectionsSearched, out.QueryForFilter, out.ContextKind, out.ContextRef, fmt.Errorf("%s", out.Error)
	}
	return out.Context, out.ChunkIDs, out.SearchCollection, out.CollectionsSearched, out.QueryForFilter, out.ContextKind, out.ContextRef, nil
}

// GetFullContextByRef calls mcp-read /mcp/full_context for "collection:chunk_id".
func (s *Server) GetFullContextByRef(ctx context.Context, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	collection, chunkID := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if collection == "" || chunkID == "" {
		return "", false
	}
	payload, _ := json.Marshal(map[string]string{"chunk_id": chunkID, "collection": collection})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.McpReadURL+"/mcp/full_context", bytes.NewReader(payload))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return "", false
	}
	defer resp.Body.Close()
	var out struct {
		Context string `json:"context"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		return "", false
	}
	return out.Context, true
}

// GetFullContextByChunkIDs возвращает полный контекст из core.document_context для переданных chunk_id (при ≤2 чанках в Qdrant подставляем полные тексты документов).
func (s *Server) GetFullContextByChunkIDs(ctx context.Context, chunkIDs []string) (string, bool) {
	if len(chunkIDs) == 0 {
		return "", false
	}
	rows, err := s.Pool.Query(ctx, `SELECT context FROM core.document_context WHERE chunk_id = ANY($1) ORDER BY chunk_id`, chunkIDs)
	if err != nil {
		return "", false
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var ctx string
		if err := rows.Scan(&ctx); err != nil {
			continue
		}
		t := strings.TrimSpace(ctx)
		if t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n\n"), true
}

// GetAngelNamesList возвращает отсортированный список имён из core.angel_names (для формата «name all»: список + дополнение от LLM).
func (s *Server) GetAngelNamesList(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT name FROM core.angel_names WHERE trim(name) != '' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			continue
		}
		n = strings.TrimSpace(n)
		if n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}

// GetAngelNamesFromPostgres returns context: количество имён (цифрой), затем все имена ангелов из core.angel_names + document_context (для «name all»).
func (s *Server) GetAngelNamesFromPostgres(ctx context.Context) (string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT an.name, COALESCE(dc.context, '')
		FROM core.angel_names an
		LEFT JOIN core.document_context dc ON an.chunk_id = dc.chunk_id
		ORDER BY an.name
	`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var bld strings.Builder
	var count int
	for rows.Next() {
		var name, context string
		if err := rows.Scan(&name, &context); err != nil {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		count++
		bld.WriteString("Имя: ")
		bld.WriteString(name)
		bld.WriteString("\n")
		if strings.TrimSpace(context) != "" {
			bld.WriteString(strings.TrimSpace(context))
			bld.WriteString("\n\n")
		}
	}
	body := bld.String()
	if count == 0 {
		return "Всего имён: 0\n", nil
	}
	return "Всего имён: " + fmt.Sprintf("%d", count) + "\n\n" + body, nil
}

// toNullString for DB nullable string
func toNullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

