// FastNote markdown renderer — shared by preview and exports.
//
// Pure Go, no external dependencies.  Implements the feature table of
// FASTNOTE_SPECIFICATION.md section 4.  All source text is HTML-escaped
// before any inline formatting is applied, so embedded <script> cannot
// execute.

package main

import (
	"regexp"
	"strings"
	"time"
)

// ---------------------------------------------------------------- inline

var inlineRE = regexp.MustCompile(
	"(`[^`]+`)" + // 1 inline code
		"|(\\$\\$[^$]+\\$\\$)" + // 2 block math (inline position)
		"|(\\$[^$]+\\$)" + // 3 inline math
		"|(\\[\\[([^\\]|]+)(?:\\|[^\\]|]+)?\\]\\])" + // 4,5 wiki link
		"|(!\\[([^\\]]*)\\]\\(([^)\\s]+)(?:\\s+[^)]*)?\\))" + // 6,7,8 image
		"|(\\[([^\\]]*)\\]\\(([^)\\s]+)(?:\\s+[^)]*)?\\))" + // 9,10,11 link
		"|(\\*\\*([^*]+)\\*\\*)" + // 12,13 bold
		"|(~~([^~]+)~~)" + // 14,15 strike
		"|(\\*([^*\\n]+)\\*)" + // 16,17 italic *
		"|(_([^_\\n]+)_)") // 18,19 italic _

var slugRE = regexp.MustCompile(`[^a-zA-Z0-9\x80-\x{fffd}]+`)

var escReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func esc(text string) string { return escReplacer.Replace(text) }

func slug(text string) string {
	s := slugRE.ReplaceAllString(strings.ToLower(text), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "section"
	}
	return s
}

// resolveWiki resolves [[Name]] to a note path where one exists, else name.
func resolveWiki(target, baseDir string) string {
	base := target
	for _, ext := range appExtensions {
		if strings.HasSuffix(strings.ToLower(target), ext) {
			base = strings.TrimSuffix(target, ext)
			break
		}
	}
	for _, ext := range appExtensions {
		c := base + ext
		if baseDir != "" {
			c = baseDir + "/" + base + ext
		}
		if fileExists(c) {
			return c
		}
	}
	return target
}

func renderInline(text, baseDir string) string {
	var out strings.Builder
	pos := 0
	for _, m := range inlineRE.FindAllStringSubmatchIndex(text, -1) {
		start, end := m[0], m[1]
		out.WriteString(esc(text[pos:start]))
		g := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return text[m[2*i]:m[2*i+1]]
		}
		switch {
		case g(1) != "":
			out.WriteString("<code>" + esc(g(1)[1:len(g(1))-1]) + "</code>")
		case g(2) != "":
			out.WriteString(`<span class="math">\(` + esc(g(2)[2:len(g(2))-2]) + `\)</span>`)
		case g(3) != "":
			out.WriteString(`<span class="math">\(` + esc(g(3)[1:len(g(3))-1]) + `\)</span>`)
		case g(4) != "":
			target := g(5)
			resolved := resolveWiki(target, baseDir)
			out.WriteString(`<a class="wiki" href="` + esc(resolved) + `">` + esc(target) + `</a>`)
		case g(6) != "":
			out.WriteString(`<img alt="` + esc(g(7)) + `" src="` + esc(g(8)) + `">`)
		case g(9) != "":
			out.WriteString(`<a href="` + esc(g(11)) + `">` + esc(g(10)) + `</a>`)
		case g(12) != "":
			out.WriteString("<strong>" + renderInline(g(13), baseDir) + "</strong>")
		case g(14) != "":
			out.WriteString("<del>" + esc(g(15)) + "</del>")
		case g(16) != "":
			out.WriteString("<em>" + renderInline(g(17), baseDir) + "</em>")
		case g(18) != "":
			out.WriteString("<em>" + renderInline(g(19), baseDir) + "</em>")
		}
		pos = end
	}
	out.WriteString(esc(text[pos:]))
	return out.String()
}

// ---------------------------------------------------------------- blocks

var (
	headingRE   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	hrRE        = regexp.MustCompile(`^-{3,}$|^\*{3,}$|^_{3,}$`)
	listItemRE  = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(.*)$`)
	taskRE      = regexp.MustCompile(`^\[([ xX])\]\s+(.*)$`)
	sepRowRE    = regexp.MustCompile(`\|.*--`)
	cellRE      = regexp.MustCompile(`\|`)
	tripleRE    = regexp.MustCompile("^```")
	plainStrip1 = regexp.MustCompile(`\*\*`)
	plainStrip2 = regexp.MustCompile(`~~`)
	plainStrip3 = regexp.MustCompile("`")
	plainStrip4 = regexp.MustCompile(`\$`)
	plainStrip5 = regexp.MustCompile(`\[\[`)
	plainStrip6 = regexp.MustCompile(`\]\]`)
	plainLink   = regexp.MustCompile(`\[\]\(([^)]*)\)`)
	plainImg    = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]*)\)`)
)

func splitHeaderRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	cells := []string{}
	buf := ""
	for _, ch := range line {
		if ch == '|' {
			cells = append(cells, strings.TrimSpace(buf))
			buf = ""
		} else {
			buf += string(ch)
		}
	}
	return append(cells, strings.TrimSpace(buf))
}

// RenderFragment renders markdown source to an HTML fragment.  A single
// pass with a simple state machine — fast on huge input.
func RenderFragment(text, baseDir string) string {
	lines := strings.Split(text, "\n")
	out := []string{}
	headings := [][2]string{} // level, title

	flushList := func(buf [][3]string, kind string) {
		if len(buf) == 0 {
			return
		}
		tag := "ul"
		if kind == "ol" {
			tag = "ol"
		}
		out = append(out, "<"+tag+">")
		for _, item := range buf {
			indent, checkbox, content := item[0], item[1], item[2]
			inner := renderInline(content, baseDir)
			if checkbox != "" {
				inner = `<input type="checkbox" ` + checkbox + ` disabled> ` + inner
			}
			pad := strings.Repeat("  ", mustAtoi(indent)/2)
			out = append(out, pad+"<li>"+inner+"</li>")
		}
		out = append(out, "</"+tag+">")
	}

	flushTable := func(tableBuf []string) {
		if len(tableBuf) == 0 {
			return
		}
		out = append(out, "<table>")
		for r, row := range tableBuf {
			tag := "td"
			if r == 0 {
				tag = "th"
			}
			cells := splitHeaderRow(row)
			var sb strings.Builder
			sb.WriteString("<tr>")
			for _, c := range cells {
				sb.WriteString("<" + tag + ">" + renderInline(c, baseDir) + "</" + tag + ">")
			}
			sb.WriteString("</tr>")
			out = append(out, sb.String())
		}
		out = append(out, "</table>")
	}

	flushQuote := func(quoteBuf []string) {
		if len(quoteBuf) == 0 {
			return
		}
		body := make([]string, 0, len(quoteBuf))
		for _, q := range quoteBuf {
			body = append(body, renderInline(q, baseDir))
		}
		out = append(out, "<blockquote>"+strings.Join(body, "<br>\n")+"</blockquote>")
	}

	flushCode := func(codeLang string, codeBuf []string) {
		if len(codeBuf) == 0 {
			return
		}
		langCls := ""
		if codeLang != "" {
			langCls = ` class="language-` + esc(codeLang) + `"`
		}
		out = append(out, "<pre><code"+langCls+">"+esc(strings.Join(codeBuf, "\n"))+"</code></pre>")
	}

	var (
		listBuf  [][3]string
		listKind string
		quoteBuf []string
		tableBuf []string
	)
	inCode, inQuote, inTable := false, false, false
	codeLang, codeBuf := "", []string{}

	for i, raw := range lines {
		if inCode {
			if strings.HasPrefix(strings.TrimSpace(raw), "```") {
				flushCode(codeLang, codeBuf)
				codeBuf = nil
				inCode = false
				continue
			}
			codeBuf = append(codeBuf, raw)
			continue
		}

		stripped := strings.TrimSpace(raw)

		if strings.HasPrefix(stripped, "```") {
			inCode = true
			codeLang = strings.TrimSpace(stripped[3:])
			codeBuf = nil
			continue
		}

		// headings
		if m := headingRE.FindStringSubmatch(stripped); m != nil {
			flushList(listBuf, listKind)
			listBuf = nil
			title := strings.TrimSpace(m[2])
			level := itoa(len(m[1]))
			headings = append(headings, [2]string{"h" + level, title})
			out = append(out, "<h"+level+` id="`+slug(title)+`">`+renderInline(title, baseDir)+"</h"+level+">")
			continue
		}

		// horizontal rule
		if hrRE.MatchString(stripped) {
			flushTable(tableBuf)
			tableBuf = nil
			flushQuote(quoteBuf)
			quoteBuf = nil
			flushList(listBuf, listKind)
			listBuf = nil
			out = append(out, "<hr>")
			continue
		}

		// blockquote
		if strings.HasPrefix(stripped, ">") {
			flushList(listBuf, listKind)
			listBuf = nil
			flushTable(tableBuf)
			tableBuf = nil
			inQuote = true
			quoteBuf = append(quoteBuf, strings.TrimSpace(strings.TrimLeft(stripped, ">")))
			continue
		}

		// table: a pipe row starts a table only if the next row is a separator
		if !inTable && strings.HasPrefix(stripped, "|") {
			nxt := ""
			if i+1 < len(lines) {
				nxt = strings.TrimSpace(lines[i+1])
			}
			if strings.HasPrefix(nxt, "|") && sepRowRE.MatchString(nxt) {
				inTable = true
				tableBuf = []string{stripped}
				continue
			}
		}
		if inTable && strings.HasPrefix(stripped, "|") {
			if strings.Contains(stripped, "--") {
				continue // separator row
			}
			tableBuf = append(tableBuf, stripped)
			continue
		}
		if inTable {
			flushTable(tableBuf)
			tableBuf = nil
			inTable = false
		}

		// lists
		if lm := listItemRE.FindStringSubmatch(raw); lm != nil {
			flushQuote(quoteBuf)
			quoteBuf = nil
			flushTable(tableBuf)
			tableBuf = nil
			indent := len(lm[1])
			marker := lm[2]
			content := lm[3]
			checkbox := ""
			if tm := taskRE.FindStringSubmatch(content); tm != nil {
				if strings.ToLower(tm[1]) == "x" {
					checkbox = "checked"
				}
				content = tm[2]
			}
			kind := "-"
			if isOrdered(marker) {
				kind = "ol"
			}
			if len(listBuf) == 0 || listKind != kind {
				flushList(listBuf, listKind)
				listBuf = nil
				listKind = kind
			}
			listBuf = append(listBuf, [3]string{fmtInt(indent), checkbox, content})
			continue
		}
		if len(listBuf) > 0 {
			flushList(listBuf, listKind)
			listBuf = nil
		}

		// paragraph / blank line
		if stripped == "" {
			flushQuote(quoteBuf)
			quoteBuf = nil
			if len(tableBuf) > 0 {
				flushTable(tableBuf)
				tableBuf = nil
			}
			continue
		}
		if inQuote {
			quoteBuf = append(quoteBuf, raw)
			continue
		}
		flushQuote(quoteBuf)
		quoteBuf = nil
		out = append(out, "<p>"+renderInline(stripped, baseDir)+"</p>")
	}

	if inCode {
		flushCode(codeLang, codeBuf)
	}
	flushList(listBuf, listKind)
	flushQuote(quoteBuf)
	flushTable(tableBuf)

	body := strings.Join(out, "\n")
	if len(headings) > 0 {
		tocItems := make([]string, 0, len(headings))
		for _, h := range headings {
			tocItems = append(tocItems,
				`<li class="toc-`+h[0]+`"><a href="#`+slug(h[1])+`">`+esc(h[1])+`</a></li>`)
		}
		toc := `<nav class="toc" id="toc"><h2>Table of Contents</h2><ol>` +
			strings.Join(tocItems, "\n") + "</ol></nav>"
		if strings.Contains(body, "[[TOC]]") {
			body = strings.Replace(body, "[[TOC]]", toc, 1)
		} else {
			body = toc + "\n" + body
		}
	}
	return body
}

func mustAtoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func isOrdered(marker string) bool {
	return len(marker) > 0 && marker[0] >= '0' && marker[0] <= '9'
}

func fmtInt(i int) string {
	return itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}

// ---------------------------------------------------------------- themes

type themeColors struct {
	bg, fg, codeBG, border, accent, tocBG string
}

var themes = map[string]themeColors{
	"light": {"#ffffff", "#1f2328", "#f3f4f6", "#d8dee4", "#0969da", "#f6f8fa"},
	"dark":  {"#0d1117", "#e6edf3", "#161b22", "#30363d", "#4493f8", "#161b22"},
}

func buildStyle(theme, customCSS string) string {
	t := themes[theme]
	if t.bg == "" {
		t = themes["light"]
	}
	css := "body { background: " + t.bg + "; color: " + t.fg +
		";\n       font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;" +
		"\n       max-width: 860px; margin: 0 auto; padding: 1.5em; line-height: 1.55; }\n" +
		"h1, h2, h3, h4, h5, h6 { border-bottom: 1px solid " + t.border + "; padding-bottom: .2em; }\n" +
		"a { color: " + t.accent + "; }\n" +
		"code, pre { background: " + t.codeBG + "; border-radius: 6px; }\n" +
		"code { padding: .15em .35em; }\n" +
		"pre { padding: .8em 1em; overflow-x: auto; }\n" +
		"pre code { background: none; padding: 0; }\n" +
		"blockquote { border-left: 4px solid " + t.border + "; margin-left: 0; padding-left: 1em;\n" +
		"             color: " + t.fg + "; opacity: .85; }\n" +
		"table { border-collapse: collapse; margin: 1em 0; }\n" +
		"th, td { border: 1px solid " + t.border + "; padding: .35em .7em; }\n" +
		"th { background: " + t.codeBG + "; }\n" +
		".math { font-family: 'STIX Two Math', 'Cambria Math', serif; }\n" +
		"nav.toc { background: " + t.tocBG + "; border: 1px solid " + t.border +
		";\n          border-radius: 8px; padding: .6em 1.2em; margin: 1em 0; }\n" +
		"nav.toc ol { margin: 0; padding-left: 1.4em; }\n" +
		"li:has(> input[type=\"checkbox\"]) { list-style: none; margin-left: -1.2em; }\n" +
		"img { max-width: 100%; }\n"
	if customCSS != "" {
		css += "\n/* injected custom css */\n" + SanitizeCSS(customCSS)
	}
	return css
}

var badCSSRE = regexp.MustCompile(`[^ {}a-zA-Z0-9#%.,:;_\-/()*"'\[\]]`)

// SanitizeCSS only allows harmless style rules; anything executable-shaped
// is dropped: no '<' or '>' (can't smuggle markup), no 'url(' (can't load
// resources), no 'expression' (legacy IE execution), length-capped.
func SanitizeCSS(css string) string {
	if len(css) > 8192 {
		css = css[:8192]
	}
	lower := strings.ToLower(css)
	if strings.ContainsAny(css, "<>") || strings.Contains(lower, "url(") ||
		strings.Contains(lower, "expression") {
		return ""
	}
	if badCSSRE.MatchString(css) {
		return ""
	}
	return css
}

// RenderPage renders a full standalone HTML document (FR-7).
func RenderPage(text, title, theme, customCSS string) string {
	body := RenderFragment(text, "")
	return "<!DOCTYPE html>\n" +
		"<html lang=\"en\">\n" +
		"<head>\n" +
		"<meta charset=\"utf-8\">\n" +
		"<title>" + esc(title) + "</title>\n" +
		"<style>\n" + buildStyle(theme, customCSS) + "\n</style>\n" +
		"</head>\n" +
		"<body>\n" + body + "\n</body>\n</html>\n"
}

var docTitleRE = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)

func docTitle(text string) string {
	m := docTitleRE.FindStringSubmatch(text)
	if m == nil {
		return "FastNote"
	}
	return esc(strings.TrimSpace(m[1]))
}

// RenderPlain is the pseudo-rendered plain text for the preview pane and
// PDF.  Distinct from the raw source: headings upper-cased with rules,
// markup stripped, task state shown, code fenced.
func RenderPlain(text string) string {
	out := []string{}
	inCode := false
	for _, line := range strings.Split(text, "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "```") {
			if !inCode {
				out = append(out, "┌─ code ─────────────────────────────")
			} else {
				out = append(out, "└─ end code ────────────────────────")
			}
			inCode = !inCode
			continue
		}
		if inCode {
			out = append(out, "    "+line)
			continue
		}
		if m := headingRE.FindStringSubmatch(s); m != nil {
			if len(m[1]) == 1 {
				out = append(out, "\n"+strings.Repeat("═", 60)+"\n"+
					strings.ToUpper(m[2])+"\n"+strings.Repeat("═", 60))
			} else {
				out = append(out, strings.ToUpper(m[2]))
			}
			continue
		}
		if lm := listItemRE.FindStringSubmatch(s); lm != nil {
			content := lm[3]
			if tm := taskRE.FindStringSubmatch(content); tm != nil {
				box := "[ ]"
				if strings.ToLower(tm[1]) == "x" {
					box = "[x]"
				}
				out = append(out, "  "+box+" "+tm[2])
			} else {
				out = append(out, "  • "+content)
			}
			continue
		}
		if hrRE.MatchString(s) {
			out = append(out, strings.Repeat("─", 60))
			continue
		}
		if strings.HasPrefix(s, ">") {
			out = append(out, "  ▌ "+strings.TrimSpace(strings.TrimLeft(s, ">")))
			continue
		}
		if s != "" {
			plain := s
			for _, p := range []*regexp.Regexp{plainStrip1, plainStrip2, plainStrip3,
				plainStrip4, plainStrip5, plainStrip6} {
				plain = p.ReplaceAllString(plain, "")
			}
			plain = plainLink.ReplaceAllString(plain, "($1)")
			plain = plainImg.ReplaceAllString(plain, "[img: $1]")
			out = append(out, plain)
		} else {
			out = append(out, "")
		}
	}
	return strings.Join(out, "\n")
}

// MeasureLargeDocument synthesizes the spec's worst case (60 KB, ~1000
// headings) and times the renderer.
func MeasureLargeDocument() float64 {
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString(itoa(i) + "\n")
	}
	sb.Reset()
	for i := 0; i < 1000; i++ {
		sb.WriteString("# Heading " + itoa(i) + "\n")
		sb.WriteString("Some **body** text with *italics* and `code` and $x^2$.\n\n")
	}
	start := time.Now()
	RenderFragment(sb.String(), "")
	return time.Since(start).Seconds()
}