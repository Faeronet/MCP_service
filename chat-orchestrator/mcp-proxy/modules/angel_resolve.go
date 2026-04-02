package modules

import (
	"context"
	"strings"
)

// NormalizeSchedulerChunkID strips optional "collection:chunk_id" / "ref:" prefix; returns bare chunk_id for DB.
func NormalizeSchedulerChunkID(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if i := strings.LastIndex(ref, ":"); i >= 0 && i < len(ref)-1 {
		tail := strings.TrimSpace(ref[i+1:])
		if tail != "" {
			return tail
		}
	}
	return ref
}

// AngelNameFromChunkID loads `name` from core.angel_names (same field used / synced with Qdrant chunk payload).
func (s *Server) AngelNameFromChunkID(ctx context.Context, chunkID string) string {
	id := NormalizeSchedulerChunkID(chunkID)
	if id == "" || s.Pool == nil {
		return ""
	}
	var name string
	err := s.Pool.QueryRow(ctx, `
		SELECT trim(name) FROM core.angel_names WHERE chunk_id = $1
	`, id).Scan(&name)
	if err != nil || strings.TrimSpace(name) == "" {
		return ""
	}
	return strings.TrimSpace(name)
}

// ResolveAngelImagePathForScheduler tries DB name (Qdrant/chunk `name`), then request angel_name, with string variants.
func (s *Server) ResolveAngelImagePathForScheduler(ctx context.Context, chunkID, angelNameFallback string) string {
	dir := ""
	dbName := s.AngelNameFromChunkID(ctx, chunkID)
	return ResolveAngelImagePathAny(dir, AngelNameCandidateList(dbName, angelNameFallback)...)
}
