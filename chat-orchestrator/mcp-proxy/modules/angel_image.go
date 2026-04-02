package modules

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const defaultAngelsImgDir = "/angels-img"

// angelNameToIndex: normalized name (RU or Latin) -> pic index 1..72
var angelNameToIndex map[string]int

func init() {
	angelNameToIndex = make(map[string]int)
	add := func(n int, names ...string) {
		for _, raw := range names {
			k := normalizeAngelName(raw)
			if k != "" {
				angelNameToIndex[k] = n
			}
		}
	}
	add(1, "Вехюиах", "Vehuiah")
	add(2, "Иелиель", "Jeliel")
	add(3, "Ситаель", "Sitael")
	add(4, "Элемиах", "Elemiah")
	add(5, "Махазиах", "Mahasiah")
	add(6, "Лелахель", "Lelahel")
	add(7, "Ахаиах", "Achaiah")
	add(8, "Кахетель", "Cahetel")
	add(9, "Хазиель", "Haziel")
	add(10, "Аладиах", "Aladiah")
	add(11, "Лауиах", "Lauviah")
	add(12, "Хахаиах", "Hahaiah")
	add(13, "Иезелель", "Iezalel")
	add(14, "Мебахель", "Mebahel")
	add(15, "Хариель", "Hariel")
	add(16, "Хакамиах", "Hekamiah")
	add(17, "Левиах", "Leviah")
	add(18, "Калиель", "Caliel")
	add(19, "Леувиах", "Leuviah")
	add(20, "Пахалиах", "Pahaliah")
	add(21, "Нелькаель", "Nelkhael")
	add(22, "Иеиаиель", "Yeiayel")
	add(23, "Мелахель", "Melahel")
	add(24, "Хахюиах")
	add(25, "Нитхаиах", "Nith-Haiah", "Nithhaiah")
	add(26, "Хааиах", "Haaiah")
	add(27, "Иератхель", "Yerathel")
	add(28, "Сехиах", "Seheiah")
	add(29, "Рейиель", "Reiyel")
	add(30, "Ормаёль", "Omael", "Omadamabiahel", "OmaDamabiahel")
	add(31, "Лекабель", "Lecabel")
	add(32, "Васариах", "Vasariah")
	add(33, "Иехюиах", "Yehuiah")
	add(34, "Лехахиах", "Lehahiah")
	add(35, "Кавакиах", "Khavakhiah")
	add(36, "Манадель", "Menadel")
	add(37, "Аниель", "Aniel")
	add(38, "Хаамиах", "Haamiah")
	add(39, "Рехаёль", "Rehael")
	add(40, "Иейазель", "Ieiazel")
	add(41, "Хахахель", "Hahahel")
	add(42, "Микаёль", "Mikael")
	add(43, "Вевалиах", "Veuliah")
	add(44, "Иелахиах", "Yelahiah")
	add(45, "Сеалиах", "Sealiah")
	add(46, "Ариель", "Ariel")
	add(47, "Асалиах", "Asaliah", "Assaliah")
	add(48, "Михаёль", "Mihael")
	add(49, "Вехюель", "Vehuel")
	add(50, "Даниель", "Daniel")
	add(51, "Хахасиах", "Hahasiah")
	add(52, "Имамиах", "Imamiah")
	add(53, "Нанаёль", "Nanael")
	add(54, "Нитаёль", "Nithael")
	add(55, "Мебаиах", "Mebahiah")
	add(56, "Поиель", "Poyel")
	add(57, "Неммамиах", "Nemamiah")
	add(58, "Иеиалель", "Yeialel")
	add(59, "Харахель", "Harahel")
	add(60, "Мизраель", "Mitzrael")
	add(61, "Умабель", "Umabel")
	add(62, "Иаххель", "Iahhel")
	add(63, "Анаюель", "Anauel")
	add(64, "Мехиель", "Mehiel")
	add(65, "Дамабиах", "Damabiah")
	add(66, "Манакель", "Manakel")
	add(67, "Эйаёль", "Eyael")
	add(68, "Хабюиах", "Habuhiah")
	add(69, "Рохель", "Rochel")
	add(70, "Иабамиах", "Jabamiah")
	add(71, "Хаиаиель", "Haiaiel")
	add(72, "Мюмиах", "Mumiah")
}

func normalizeAngelName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r == 'ё' {
			r = 'е'
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			continue
		}
		// skip spaces/punctuation for fuzzy match
	}
	return b.String()
}

// AngelPicIndex resolves 1..72 from display name (may be RU or Latin).
func AngelPicIndex(angelName string) int {
	n := angelNameToIndex[normalizeAngelName(angelName)]
	if n != 0 {
		return n
	}
	// Substring: long reminder text might prefix with name
	norm := normalizeAngelName(angelName)
	for k, v := range angelNameToIndex {
		if len(k) >= 4 && strings.Contains(norm, k) {
			return v
		}
	}
	return 0
}

// AngelNameCandidateList expands names for image lookup (parentheses, slash, dedup).
func AngelNameCandidateList(names ...string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		for _, c := range expandAngelNameStrings(raw) {
			k := strings.ToLower(strings.TrimSpace(c))
			if k == "" {
				continue
			}
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, strings.TrimSpace(c))
		}
	}
	for _, n := range names {
		add(n)
	}
	return out
}

func expandAngelNameStrings(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := []string{s}
	// "Before (Inside)"
	if i := strings.Index(s, "("); i >= 0 {
		j := strings.LastIndex(s, ")")
		if j > i+1 {
			before := strings.TrimSpace(s[:i])
			inside := strings.TrimSpace(s[i+1 : j])
			if before != "" {
				out = append(out, before)
			}
			if inside != "" {
				out = append(out, inside)
			}
		}
	}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == '/' || r == '|' || r == '•'
	}) {
		p := strings.TrimSpace(part)
		if p != "" && p != s {
			out = append(out, p)
		}
	}
	return out
}

// ResolveAngelImagePathAny returns first existing image for any candidate name.
func ResolveAngelImagePathAny(dir string, names ...string) string {
	for _, n := range names {
		if p := ResolveAngelImagePath(dir, n); p != "" {
			return p
		}
	}
	return ""
}

// ResolveAngelImagePath returns path to picN.jpg (or picN_1.jpg) or "".
func ResolveAngelImagePath(dir, angelName string) string {
	if dir == "" {
		dir = strings.TrimSpace(os.Getenv("ANGELS_IMG_DIR"))
	}
	if dir == "" {
		dir = defaultAngelsImgDir
	}
	n := AngelPicIndex(angelName)
	if n <= 0 {
		return ""
	}
	base := filepath.Join(dir, "pic"+itoa(n))
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		p := base + ext
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	alt := filepath.Join(dir, "pic"+itoa(n)+"_1.jpg")
	if st, err := os.Stat(alt); err == nil && !st.IsDir() {
		return alt
	}
	return ""
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	x := n
	for x > 0 {
		i--
		buf[i] = byte('0' + x%10)
		x /= 10
	}
	return string(buf[i:])
}

// TelegramPhotoCaptionMax is Telegram's caption limit for photos.
const TelegramPhotoCaptionMax = 1024
