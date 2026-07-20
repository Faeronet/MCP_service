package modules

import (
	"fmt"
	"testing"
)

func TestUserFacingProxyError(t *testing.T) {
	if got := userFacingProxyError(0, fmt.Errorf("timeout")); got != userFriendlyErrorRU {
		t.Fatalf("expected friendly message, got %q", got)
	}
	if got := userFacingProxyError(1, fmt.Errorf("timeout")); got == userFriendlyErrorRU {
		t.Fatal("expected debug detail in debug mode")
	}
}

func TestSanitizeProxyReplyText(t *testing.T) {
	tech := "Ответ модели пустой (часто весь текст был во внутреннем блоке рассуждения)."
	if got := sanitizeProxyReplyText(0, tech); got != userFriendlyErrorRU {
		t.Fatalf("expected friendly rewrite, got %q", got)
	}
	if got := sanitizeProxyReplyText(1, tech); got != tech {
		t.Fatalf("debug mode should keep original text")
	}
}
