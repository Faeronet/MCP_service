package modules

import (
	"encoding/json"
	"strings"
)

const noteScheduleBridgeKey = "__mcp_schedule_from_note__"

// IsNoteScheduleBridgeFile reports whether raw bytes are the note export JSON that
// mcp-proxy forwards to the scheduler (same marker and rules as maybeExtractBridgePayload).
func IsNoteScheduleBridgeFile(data []byte) bool {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return false
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return false
	}
	text = text[start : end+1]
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		return false
	}
	keyRaw, ok := obj[noteScheduleBridgeKey]
	if !ok {
		return false
	}
	var enabled bool
	if err := json.Unmarshal(keyRaw, &enabled); err != nil || !enabled {
		return false
	}
	return true
}
