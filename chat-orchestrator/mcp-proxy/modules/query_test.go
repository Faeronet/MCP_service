package modules

import (
	"fmt"
	"strings"
	"testing"
)

func TestStripThink(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Привет", "Привет"},
		{"`think`скрыто`/think` видно", "видно"},
		{"<think>xxx</think>ответ", "ответ"},
		{"<think>xxx</think>ответ", "ответ"},
	}
	for _, tc := range tests {
		got := StripThink(tc.in)
		if got != tc.want {
			t.Errorf("StripThink(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUserFacingChatError(t *testing.T) {
	s := &Server{DebugMode: 0}
	if got := userFacingChatError(s, fmt.Errorf("timeout")); got != userFriendlyErrorRU {
		t.Fatalf("expected friendly message, got %q", got)
	}
	s.DebugMode = 1
	if got := userFacingChatError(s, fmt.Errorf("timeout")); !strings.Contains(got, "timeout") {
		t.Fatalf("expected debug detail, got %q", got)
	}
}
