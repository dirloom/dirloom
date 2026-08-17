package render

import (
	"fmt"
	"strings"
	"unicode"
)

func safeDiagramText(value string) string {
	var result strings.Builder
	for _, r := range value {
		switch r {
		case '\t':
			result.WriteString(`\t`)
		case '\n':
			result.WriteString(`\n`)
		case '\r':
			result.WriteString(`\r`)
		default:
			if unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp) || isDiagramBidiControl(r) {
				_, _ = fmt.Fprintf(&result, `\u{%04X}`, r)
			} else {
				result.WriteRune(r)
			}
		}
	}
	return result.String()
}

func escapeMermaidLabel(value string) string {
	value = safeDiagramText(value)
	replacer := strings.NewReplacer(
		"&", "&amp;",
		`"`, "&quot;",
		"<", "&lt;",
		">", "&gt;",
		"`", "&#96;",
		`\`, "&#92;",
	)
	return replacer.Replace(value)
}

func escapeDOTLabel(value string) string {
	value = safeDiagramText(value)
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func escapeD2Label(value string) string {
	value = safeDiagramText(value)
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func isDiagramBidiControl(r rune) bool {
	switch r {
	case '\u061c', '\u200e', '\u200f', '\u202a', '\u202b', '\u202c', '\u202d', '\u202e', '\u2066', '\u2067', '\u2068', '\u2069':
		return true
	default:
		return false
	}
}
