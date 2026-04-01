package modules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var reDateMonthRu = regexp.MustCompile(`(\d{1,2})\s+(января|февраля|марта|апреля|мая|июня|июля|августа|сентября|октября|ноября|декабря)`)
var reDateDot = regexp.MustCompile(`(\d{1,2})\.(\d{1,2})(?:\.(\d{2,4}))?`)

func extractDateFromQuery(query string) (dateStr string, ok bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return "", false
	}
	if m := reDateMonthRu.FindString(q); m != "" {
		return m, true
	}
	if m := reDateDot.FindString(q); m != "" {
		return m, true
	}
	return "", false
}

func queryDayLessThan10(dateStr string) bool {
	day := parseDayFromDateStr(dateStr)
	return day > 0 && day < 10
}

func dateStrToAlternateForm(dateStr string) string {
	dateStr = strings.TrimSpace(dateStr)
	if reDateMonthRu.MatchString(dateStr) {
		sub := reDateMonthRu.FindStringSubmatch(dateStr)
		if len(sub) >= 3 {
			day, _ := strconv.Atoi(sub[1])
			monthNames := []string{"", "января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
			for i, name := range monthNames {
				if i > 0 && name == sub[2] {
					return fmt.Sprintf("%d.%02d", day, i)
				}
			}
		}
	}
	if reDateDot.MatchString(dateStr) {
		sub := reDateDot.FindStringSubmatch(dateStr)
		if len(sub) >= 3 {
			day, _ := strconv.Atoi(sub[1])
			month, _ := strconv.Atoi(sub[2])
			monthNames := []string{"", "января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
			if month >= 1 && month <= 12 {
				return fmt.Sprintf("%d %s", day, monthNames[month])
			}
		}
	}
	return ""
}

func chunkContainsDate(chunkText, dateStr string) bool {
	if strings.Contains(chunkText, dateStr) {
		return true
	}
	if alt := dateStrToAlternateForm(dateStr); alt != "" && strings.Contains(chunkText, alt) {
		return true
	}
	return false
}

func parseDayFromDateStr(s string) int {
	s = strings.TrimSpace(s)
	if m := reDateMonthRu.FindStringSubmatch(s); len(m) >= 2 {
		if d, err := strconv.Atoi(m[1]); err == nil && d >= 1 && d <= 31 {
			return d
		}
	}
	if m := reDateDot.FindStringSubmatch(s); len(m) >= 2 {
		if d, err := strconv.Atoi(m[1]); err == nil && d >= 1 && d <= 31 {
			return d
		}
	}
	return 0
}

func chunkHasDateWithDayLessThan10(text string) bool {
	for _, sub := range reDateMonthRu.FindAllStringSubmatch(text, -1) {
		if len(sub) >= 2 {
			if d, err := strconv.Atoi(sub[1]); err == nil && d >= 1 && d < 10 {
				return true
			}
		}
	}
	for _, sub := range reDateDot.FindAllStringSubmatch(text, -1) {
		if len(sub) >= 2 {
			if d, err := strconv.Atoi(sub[1]); err == nil && d >= 1 && d < 10 {
				return true
			}
		}
	}
	return false
}
