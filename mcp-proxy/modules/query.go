package modules

import (
	"regexp"
	"strconv"
	"strings"
)

var reThinkBlock = regexp.MustCompile(`(?is)<think[^>]*>.*?` + "</think>")

var angelSynonymsForDetection = []string{
	"ангелы-хранители", "ангелов-хранителей", "ангел-хранитель", "ангела-хранителя",
	"ангелы", "ангелов", "ангелам", "ангелами", "ангелах",
	"ангел", "ангела", "ангелу", "ангелом", "ангеле",
	"хранители", "хранителей", "хранитель", "хранителя", "хранителю", "хранителем", "хранителе",
}

var monthVariants = []string{
	"января", "янвря", "янаря", "январь",
	"февраля", "феврля", "феварля", "февраль",
	"марта", "матра", "мрта", "март",
	"апреля", "апереля", "апрелья", "апрель",
	"мая", "май",
	"июня", "июна", "июнь",
	"июля", "июль",
	"августа", "авгста", "август",
	"сентября", "сентябрь", "сентебря", "сентябра",
	"октября", "октбря", "октябрья", "октябрь",
	"ноября", "ноебря", "оября", "ноядбоя", "ноябрь", "ноябра",
	"декабря", "декабрля", "декбаря", "декабрь",
}

var reDateMonthRu = regexp.MustCompile(`(\d{1,2})\s+(` + strings.Join(monthVariants, "|") + `)`)
var reDateDot = regexp.MustCompile(`(\d{1,2})\.(\d{1,2})(?:\.\d{2,4})?`)

var monthNameToNum = func() map[string]int {
	groups := [][]string{
		{"января", "янвря", "янаря", "январь"},
		{"февраля", "феврля", "феварля", "февраль"},
		{"марта", "матра", "мрта", "март"},
		{"апреля", "апереля", "апрелья", "апрель"},
		{"мая", "май"},
		{"июня", "июна", "июнь"},
		{"июля", "июль"},
		{"августа", "авгста", "август"},
		{"сентября", "сентябрь", "сентебря", "сентябра"},
		{"октября", "октбря", "октябрья", "октябрь"},
		{"ноября", "ноебря", "оября", "ноядбоя", "ноябрь", "ноябра"},
		{"декабря", "декабрля", "декбаря", "декабрь"},
	}
	m := make(map[string]int)
	for i, variants := range groups {
		for _, v := range variants {
			m[strings.ToLower(v)] = i + 1
		}
	}
	return m
}()

var enMonthToRu = map[string]string{
	"January": "января", "February": "февраля", "March": "марта", "April": "апреля",
	"May": "мая", "June": "июня", "July": "июля", "August": "августа",
	"September": "сентября", "October": "октября", "November": "ноября", "December": "декабря",
	"Jan": "января", "Feb": "февраля", "Mar": "марта", "Apr": "апреля",
	"Jun": "июня", "Jul": "июля", "Aug": "августа", "Sep": "сентября",
	"Oct": "октября", "Nov": "ноября", "Dec": "декабря",
}

var enMonthToRuLower = func() map[string]string {
	m := make(map[string]string)
	for k, v := range enMonthToRu {
		m[strings.ToLower(k)] = v
	}
	return m
}()

func StripThink(s string) string {
	return strings.TrimSpace(reThinkBlock.ReplaceAllString(s, ""))
}

func HasAngelWord(s string) bool {
	lower := strings.ToLower(s)
	for _, phrase := range angelSynonymsForDetection {
		if phrase == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)(^|[^\p{L}])` + regexp.QuoteMeta(phrase) + `([^\p{L}]|$)`)
		if re.MatchString(lower) {
			return true
		}
	}
	return false
}

func MaxDaysInMonth(month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		return 29
	default:
		return 0
	}
}

func ParseDayMonthFromQuery(query string) (day, month int, ok bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, 0, false
	}
	if m := reDateMonthRu.FindStringSubmatch(q); len(m) >= 3 {
		d, err := strconv.Atoi(m[1])
		if err != nil || d < 0 || d > 31 {
			return 0, 0, false
		}
		monName := strings.ToLower(strings.TrimSpace(m[2]))
		if mon, has := monthNameToNum[monName]; has {
			return d, mon, true
		}
		return d, 0, false
	}
	if m := reDateDot.FindStringSubmatch(q); len(m) >= 3 {
		d, err1 := strconv.Atoi(m[1])
		mon, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil || mon < 1 || mon > 12 || d < 0 || d > 31 {
			return 0, 0, false
		}
		return d, mon, true
	}
	return 0, 0, false
}

func IsOnlyMonth(s string) bool {
	t := strings.TrimSpace(strings.ToLower(s))
	if t == "" {
		return false
	}
	if _, has := monthNameToNum[t]; has {
		return true
	}
	_, has := enMonthToRuLower[t]
	return has
}

func IsOnlyDay(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	n, err := strconv.Atoi(t)
	return err == nil && n >= 1 && n <= 31
}

// IsListAllAngelsRequest — запрос «назови всех ангелов которых ты знаешь» и подобные. Ловим до Prompt A, отвечаем списком.
func IsListAllAngelsRequest(msg string) bool {
	s := strings.ToLower(strings.TrimSpace(msg))
	if s == "" || !HasAngelWord(msg) {
		return false
	}
	if strings.Contains(s, "назови всех") && strings.Contains(s, "ангел") {
		return true
	}
	if strings.Contains(s, "перечисли всех") && strings.Contains(s, "ангел") {
		return true
	}
	if strings.Contains(s, "всех ангелов") && (strings.Contains(s, "которых") || strings.Contains(s, "знаешь")) {
		return true
	}
	return false
}

// IsAngelCountRequest — запрос «какое количество ангелов которых ты знаешь» и подобные. Ловим до Prompt A, отвечаем только цифрой.
func IsAngelCountRequest(msg string) bool {
	s := strings.ToLower(strings.TrimSpace(msg))
	if s == "" || !HasAngelWord(msg) {
		return false
	}
	if strings.Contains(s, "какое количество") && strings.Contains(s, "ангел") {
		return true
	}
	if strings.Contains(s, "сколько ангел") {
		return true
	}
	if (strings.Contains(s, "количество ангел") || strings.Contains(s, "число ангел")) && (strings.Contains(s, "которых") || strings.Contains(s, "знаешь")) {
		return true
	}
	return false
}

// IsMetaQuestionAboutBot — только явные вопросы «о боте», без подстрок вроде «что ты»/«what you»
// (они входят в обычные запросы: «что ты знаешь про…», «what you know about») и раньше отключали BuildContext.
func IsMetaQuestionAboutBot(s string) bool {
	q := strings.ToLower(strings.TrimSpace(s))
	if q == "" || len(q) > 200 {
		return false
	}
	if strings.Contains(q, "who are you") || strings.Contains(q, "what are you") || strings.Contains(q, "what is your name") ||
		strings.Contains(q, "tell me about yourself") {
		return true
	}
	// Не используем голые "who you" / "what you" — ловят "what you know..."
	if strings.Contains(q, "кто ты") || strings.Contains(q, "кто вы") ||
		strings.Contains(q, "как тебя зовут") || strings.Contains(q, "как вас зовут") ||
		strings.Contains(q, "расскажи о себе") || strings.Contains(q, "расскажите о себе") ||
		strings.Contains(q, "ты кто") || strings.Contains(q, "вы кто") ||
		strings.Contains(q, "что ты за") || strings.Contains(q, "что вы за") {
		return true
	}
	return false
}

func ExtractDateFromQuestion(question string) string {
	q := strings.TrimSpace(question)
	if q == "" {
		return ""
	}
	if m := reDateMonthRu.FindString(q); m != "" {
		return strings.TrimSpace(m)
	}
	if m := reDateDot.FindString(q); m != "" {
		return m
	}
	return ""
}

func TranslateMonthToRussian(s string) string {
	out := s
	for en, ru := range enMonthToRu {
		out = strings.ReplaceAll(out, en, ru)
		out = strings.ReplaceAll(out, strings.ToLower(en), ru)
	}
	return out
}
