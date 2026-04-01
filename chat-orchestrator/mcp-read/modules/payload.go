package modules

import (
	"sort"
	"strings"
)

var payloadReservedKeys = map[string]struct{}{
	"chunk_id": {}, "doc_id": {}, "version_id": {}, "section_path": {},
	"prev_chunk_id": {}, "next_chunk_id": {}, "related_chunk_ids": {}, "links": {}, "rerank_position": {},
	"text": {}, "content": {},
}

func buildTextFromPayload(p map[string]interface{}) string {
	var keys []string
	for k := range p {
		if _, reserved := payloadReservedKeys[k]; reserved {
			continue
		}
		if _, ok := p[k].(string); ok {
			keys = append(keys, k)
		} else if _, ok := p[k].([]interface{}); ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if v, ok := p[k].(string); ok && v != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v)
		} else if arr, ok := p[k].([]interface{}); ok && len(arr) > 0 {
			var names []string
			for _, x := range arr {
				if s, ok := x.(string); ok && s != "" {
					names = append(names, s)
				}
			}
			if len(names) > 0 {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(k)
				b.WriteString(": ")
				b.WriteString(strings.Join(names, ", "))
			}
		}
	}
	return b.String()
}

func buildSearchableText(p map[string]interface{}) string {
	var parts []string
	for k, v := range p {
		if _, reserved := payloadReservedKeys[k]; reserved {
			continue
		}
		if s, ok := v.(string); ok && s != "" {
			parts = append(parts, s)
		} else if arr, ok := v.([]interface{}); ok {
			for _, x := range arr {
				if s, ok := x.(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
		}
	}
	return strings.Join(parts, " ")
}

func payloadToChunkInfo(p map[string]interface{}) ChunkInfo {
	var c ChunkInfo
	if t, ok := p["text"].(string); ok && t != "" {
		c.Text = t
	} else if t, ok := p["content"].(string); ok && t != "" {
		c.Text = t
	} else {
		c.Text = buildTextFromPayload(p)
	}
	c.SearchableText = buildSearchableText(p)
	if n, ok := p["name"].(string); ok {
		c.Name = n
	}
	if id, ok := p["chunk_id"].(string); ok {
		c.ChunkID = id
	}
	if prev, ok := p["prev_chunk_id"].(string); ok {
		c.PrevID = prev
	}
	if next, ok := p["next_chunk_id"].(string); ok {
		c.NextID = next
	}
	if arr, ok := p["related_chunk_ids"].([]interface{}); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				c.RelatedIDs = append(c.RelatedIDs, s)
			}
		}
	}
	return c
}
