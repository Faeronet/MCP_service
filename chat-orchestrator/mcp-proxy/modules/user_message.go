package modules

import (
	"fmt"
)

const userFriendlyErrorRU = "Упс, с ботом что-то пошло не так. Попробуйте ещё раз."

func userFacingChatError(s *Server, err error) string {
	if s != nil && s.DebugMode == 1 {
		if err != nil {
			detail := err.Error()
			if len(detail) > 300 {
				detail = detail[:300] + "…"
			}
			return fmt.Sprintf("Ошибка LLM: %s | URL=%s, LLM_MODEL=%s", detail, s.VllmBase, s.LlmModel)
		}
		return "Ответ модели пустой (часто весь текст был во внутреннем блоке рассуждения). Повторите вопрос. Чтобы видеть полный сырой вывод, выставьте BOT_DEBUG=1 у mcp-proxy (как у tg-bot)."
	}
	return userFriendlyErrorRU
}
