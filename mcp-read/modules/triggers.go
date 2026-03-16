package modules

import (
	"regexp"
	"strings"
	"unicode"
)

var wordFilterStopwords = mkSet(
	"все", "весь", "вся", "всё", "всех", "всем", "всеми", "всего", "всей", "всею",
	"этот", "этого", "этому", "этим", "этом", "эта", "этой", "эту", "это", "эти", "этих", "этим", "этими",
	"тот", "того", "тому", "тем", "том", "та", "той", "ту", "те", "тех",
	"какой", "какого", "какому", "каким", "каком", "какая", "какой", "какую", "какие", "каких", "какими",
	"который", "которого", "которому", "которым", "котором", "которая", "которой", "которую", "которые", "которых", "которыми",
	"у", "в", "на", "по", "о", "об", "из", "с", "со", "к", "ко", "для", "при", "без", "до", "от", "за", "над", "под",
	"и", "а", "но", "как", "что", "чтобы", "чем", "не", "ни", "же", "ли", "или", "либо",
	"когда", "где", "куда", "откуда", "почему", "зачем", "как", "сколько",
)

func FilterSignificantWords(words []string) []string {
	var out []string
	for _, w := range words {
		if w == "" {
			continue
		}
		if _, ok := wordFilterStopwords[w]; !ok {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		return words
	}
	return out
}

const wordBoundaryRu = `(?:^|[^\p{L}])`
const wordBoundaryRuEnd = `(?:[^\p{L}]|$)`

func containsWordRu(s, word string) bool {
	re, err := regexp.Compile(`(?i)` + wordBoundaryRu + regexp.QuoteMeta(word) + wordBoundaryRuEnd)
	if err != nil {
		return strings.Contains(strings.ToLower(s), strings.ToLower(word))
	}
	return re.MatchString(s)
}

func CollectionForQuery(query string) string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return "chunks"
	}
	if (strings.Contains(q, "знак") && strings.Contains(q, "зодиак")) ||
		containsWordRu(q, "знак зодиака") || containsWordRu(q, "знаки зодиака") || containsWordRu(q, "знака зодиака") ||
		containsWordRu(q, "зодиак") || containsWordRu(q, "зодиака") || containsWordRu(q, "зодиаком") || containsWordRu(q, "зодиаку") {
		return "znak_zodiaka"
	}
	if containsWordRu(q, "обитание") || containsWordRu(q, "обитания") || containsWordRu(q, "обитанию") ||
		containsWordRu(q, "обитанием") || containsWordRu(q, "обитании") || containsWordRu(q, "обитает") ||
		containsWordRu(q, "живёт") || containsWordRu(q, "жил") || strings.Contains(q, "место жительства") {
		return "obitanie"
	}
	if strings.Contains(q, "качество энергии") || strings.Contains(q, "качества энергии") ||
		containsWordRu(q, "качество") || containsWordRu(q, "качества") || strings.Contains(q, "качесво") {
		return "kachestva_energii"
	}
	if strings.Contains(q, "искажение энергии") || strings.Contains(q, "искажения энергии") ||
		containsWordRu(q, "искажение") || containsWordRu(q, "искажения") {
		return "iskazheniya_energii"
	}
	if containsWordRu(q, "специфичность") || containsWordRu(q, "спецификация") ||
		containsWordRu(q, "специфичности") || containsWordRu(q, "спецификации") {
		return "specificnost"
	}
	if containsWordRu(q, "эмоциональное") || strings.Contains(q, "эмоциональн") {
		return "emocionalnoe"
	}
	if containsWordRu(q, "интеллектуальные") || containsWordRu(q, "интелектуальные") ||
		strings.Contains(q, "интеллектуальн") || strings.Contains(q, "интелектуальн") {
		return "intellektualnye"
	}
	if containsWordRu(q, "астральный дух") || (strings.Contains(q, "астральн") && strings.Contains(q, "дух")) {
		return "astralnyi_duh"
	}
	return "chunks"
}

func trimWordForMatch(w string) string {
	w = strings.TrimSpace(strings.ToLower(w))
	w = strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	return w
}

func stripTriggersByWords(s string, phraseList [][]string, wordSet map[string]struct{}) string {
	s = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(s), " ")
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	skip := make([]bool, len(words))
	normalized := make([]string, len(words))
	for i, w := range words {
		normalized[i] = trimWordForMatch(w)
	}
	for _, phrase := range phraseList {
		if len(phrase) == 0 {
			continue
		}
		phraseNorm := make([]string, len(phrase))
		for i, p := range phrase {
			phraseNorm[i] = trimWordForMatch(p)
		}
		for i := 0; i <= len(words)-len(phrase); i++ {
			match := true
			for j := 0; j < len(phrase); j++ {
				if normalized[i+j] != phraseNorm[j] {
					match = false
					break
				}
			}
			if match {
				for j := 0; j < len(phrase); j++ {
					skip[i+j] = true
				}
			}
		}
	}
	for i := range words {
		if skip[i] {
			continue
		}
		if _, ok := wordSet[normalized[i]]; ok {
			skip[i] = true
		}
	}
	var out []string
	for i := range words {
		if !skip[i] {
			out = append(out, words[i])
		}
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

func mkSet(words ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[strings.ToLower(w)] = struct{}{}
	}
	return m
}

func StripRoutingKeywords(query, collection string) string {
	if collection == "chunks" || query == "" {
		return strings.TrimSpace(query)
	}
	q := strings.TrimSpace(query)
	switch collection {
	case "znak_zodiaka":
		q = stripTriggersByWords(q,
			[][]string{
				{"знак", "зодиака"}, {"знаки", "зодиака"}, {"знаком", "зодиака"}, {"знака", "зодиака"}, {"знаку", "зодиака"},
			},
			mkSet("знак", "знаки", "знака", "знаку", "знаков", "знаком", "зодиак", "зодиака", "зодиаком", "зодиаку"))
	case "obitanie":
		q = stripTriggersByWords(q,
			[][]string{{"место", "жительства"}},
			mkSet("обитание", "обитания", "обитанию", "обитанием", "обитании", "обитает", "живёт", "жил", "живут", "жить"))
	case "kachestva_energii":
		q = stripTriggersByWords(q,
			[][]string{{"качество", "энергии"}, {"качества", "энергии"}, {"качество", "энергия"}, {"качества", "энергия"}},
			mkSet("качество", "качества", "качесво", "энергия", "энергии", "ангел", "ангела", "ангелу", "ангелом", "ангеле", "ангелы", "ангелов", "ангелам", "ангелами", "ангелах"))
	case "iskazheniya_energii":
		q = stripTriggersByWords(q,
			[][]string{{"искажение", "энергии"}, {"искажения", "энергии"}, {"искажение", "энергия"}, {"искажения", "энергия"}},
			mkSet("искажение", "искажения", "энергия", "энергии", "ангел", "ангела", "ангелу", "ангелом", "ангеле", "ангелы", "ангелов", "ангелам", "ангелами", "ангелах"))
	case "specificnost":
		q = stripTriggersByWords(q, nil, mkSet("специфичность", "спецификация", "специфичности", "спецификации"))
	case "emocionalnoe":
		q = stripTriggersByWords(q, nil, mkSet("эмоциональное", "эмоциональные", "эмоционального", "эмоциональных", "эмоциональным", "эмоциональная", "эмоциональной"))
	case "intellektualnye":
		q = stripTriggersByWords(q, [][]string{{"интеллектуальные", "способности"}, {"интелектуальные", "способности"}},
			mkSet("интеллектуальные", "интелектуальные", "интеллектуальных", "интелектуальных", "интеллектуальным", "интелектуальным"))
	case "astralnyi_duh":
		q = stripTriggersByWords(q, [][]string{{"астральный", "дух"}},
			mkSet("астральный", "астрального", "астральному", "астральным", "астральном", "дух", "духа", "духу", "духом", "духе"))
	}
	return strings.TrimSpace(strings.Join(strings.Fields(q), " "))
}
