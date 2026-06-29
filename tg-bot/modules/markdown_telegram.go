package modules

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	telegramHTMLParseMode      = "HTML"
	telegramFormattedChunkRunes = 3800 // запас под HTML-теги (лимит API — 4096)
)

var (
	reFencedCode = regexp.MustCompile("(?s)```([^\\n`]*)\n?([\\s\\S]*?)```")
	reInlineCode = regexp.MustCompile("`([^`\\n]+)`")
	reMDLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reBold2      = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reBold1      = regexp.MustCompile(`\*([^*\n]+)\*`)
	reItalic     = regexp.MustCompile(`_([^_\n]+)_`)
	reStrike     = regexp.MustCompile(`~~(.+?)~~`)
	reHeader     = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
)

func escapeTelegramHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// MarkdownToTelegramHTML converts common LLM markdown to Telegram HTML parse mode.
func MarkdownToTelegramHTML(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}

	var codeBlocks []string
	src = reFencedCode.ReplaceAllStringFunc(src, func(m string) string {
		sub := reFencedCode.FindStringSubmatch(m)
		body := m
		if len(sub) >= 3 {
			body = sub[2]
		}
		idx := len(codeBlocks)
		codeBlocks = append(codeBlocks, strings.TrimRight(body, "\n"))
		return fmt.Sprintf("\x00FB%d\x00", idx)
	})

	var inlineCodes []string
	src = reInlineCode.ReplaceAllStringFunc(src, func(m string) string {
		sub := reInlineCode.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		idx := len(inlineCodes)
		inlineCodes = append(inlineCodes, sub[1])
		return fmt.Sprintf("\x00IC%d\x00", idx)
	})

	src = escapeTelegramHTML(src)
	src = reHeader.ReplaceAllString(src, "<b>$2</b>")
	src = reMDLink.ReplaceAllString(src, `<a href="$2">$1</a>`)
	src = reBold2.ReplaceAllString(src, "<b>$1</b>")
	src = reBold1.ReplaceAllString(src, "<b>$1</b>")
	src = reItalic.ReplaceAllString(src, "<i>$1</i>")
	src = reStrike.ReplaceAllString(src, "<s>$1</s>")

	for i, code := range inlineCodes {
		src = strings.Replace(src, fmt.Sprintf("\x00IC%d\x00", i), "<code>"+escapeTelegramHTML(code)+"</code>", 1)
	}
	for i, block := range codeBlocks {
		src = strings.Replace(src, fmt.Sprintf("\x00FB%d\x00", i), "<pre><code>"+escapeTelegramHTML(block)+"</code></pre>", 1)
	}
	return src
}

type mdSegment struct {
	code  bool
	text  string
}

func parseMarkdownSegments(s string) []mdSegment {
	var segs []mdSegment
	rest := s
	for {
		idx := strings.Index(rest, "```")
		if idx < 0 {
			if rest != "" {
				segs = append(segs, mdSegment{text: rest})
			}
			break
		}
		if idx > 0 {
			segs = append(segs, mdSegment{text: rest[:idx]})
		}
		rest = rest[idx+3:]
		end := strings.Index(rest, "```")
		if end < 0 {
			segs = append(segs, mdSegment{code: true, text: rest})
			break
		}
		segs = append(segs, mdSegment{code: true, text: rest[:end]})
		rest = rest[end+3:]
	}
	return segs
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// splitMarkdownAwareChunks splits text for Telegram without breaking fenced code blocks.
func splitMarkdownAwareChunks(text string, maxRunes int) []string {
	if maxRunes < 256 {
		maxRunes = telegramFormattedChunkRunes
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if runeLen(text) <= maxRunes {
		return []string{text}
	}

	var chunks []string
	var cur strings.Builder
	curLen := 0

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		chunks = append(chunks, cur.String())
		cur.Reset()
		curLen = 0
	}

	appendText := func(part string) {
		part = strings.TrimSpace(part)
		if part == "" {
			return
		}
		for _, piece := range splitTelegramMessageChunks(part, maxRunes) {
			plen := runeLen(piece)
			if plen > maxRunes {
				flush()
				chunks = append(chunks, piece)
				continue
			}
			if curLen > 0 && curLen+plen > maxRunes {
				flush()
			}
			if cur.Len() > 0 {
				cur.WriteString("\n\n")
				curLen += 2
			}
			cur.WriteString(piece)
			curLen += plen
		}
	}

	appendCode := func(codeBody string) {
		fenced := "```" + codeBody + "```"
		flen := runeLen(fenced)
		if flen > maxRunes {
			flush()
			for _, piece := range splitTelegramMessageChunks(strings.TrimSpace(codeBody), maxRunes-8) {
				chunks = append(chunks, "```\n"+piece+"\n```")
			}
			return
		}
		if curLen > 0 && curLen+flen > maxRunes {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
			curLen += 2
		}
		cur.WriteString(fenced)
		curLen += flen
	}

	for _, seg := range parseMarkdownSegments(text) {
		if seg.code {
			appendCode(seg.text)
		} else {
			appendText(seg.text)
		}
	}
	flush()
	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
}
