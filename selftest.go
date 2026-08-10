// FastNote selftest — internal consistency checks (--selftest seam).
//
// Spec 6.1 prohibits vacuous tests: every check here exercises real
// behaviour and asserts on the outcome.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var failures []string

func check(name, detail string, ok bool) {
	if ok {
		fmt.Printf("ok   %s\n", name)
	} else {
		failures = append(failures, name)
		fmt.Printf("FAIL %s %s\n", name, detail)
	}
}

// RunSelfTest runs the whole internal suite; returns true if it passes.
func RunSelfTest() bool {
	failures = nil

	md := "# Title\n\n**bold** *italic* ~~gone~~ `code`\n\n" +
		"## Sub\n\n- a\n- [x] done\n- [ ] todo\n\n" +
		"```py\nprint(1)\n```\n\n> quote\n\n" +
		"| a | b |\n|---|---|\n| 1 | 2 |\n\n" +
		"[[Wiki]] $x^2$ and $$x+1$$ end.\n"
	frag := RenderFragment(md, "")
	check("render.headings", "", strings.Contains(frag, "<h1") && strings.Contains(frag, "<h2"))
	check("render.inline", "", strings.Contains(frag, "<strong>bold</strong>") &&
		strings.Contains(frag, "<em>italic</em>") && strings.Contains(frag, "<del>gone</del>"))
	check("render.code", "", strings.Contains(frag, "<pre><code") &&
		strings.Contains(frag, "language-py"))
	check("render.task", "", strings.Contains(frag, "checkbox") &&
		strings.Contains(frag, "checked"))
	check("render.quote", "", strings.Contains(frag, "<blockquote>quote</blockquote>"))
	check("render.table", "", strings.Contains(frag, "<table>") &&
		strings.Contains(frag, "<th>a</th>"))
	check("render.wiki", "", strings.Contains(frag, `<a class="wiki"`))
	check("render.math", "", strings.Contains(frag, `class="math"`) &&
		strings.Contains(frag, `\(`))
	check("render.toc", "", strings.Contains(frag, "Table of Contents"))

	evil := "<script>alert(1)</script>\n\n# Hi\n"
	out := RenderFragment(evil, "")
	check("render.html-escaped", "", !strings.Contains(out, "<script>") &&
		strings.Contains(out, "&lt;script&gt;"))
	check("render.css-sanitized", "",
		SanitizeCSS("p{color:red}") == "p{color:red}" &&
			SanitizeCSS("p{background:url(x)}") == "" &&
			SanitizeCSS("x<script>") == "")

	td := MeasureLargeDocument()
	check("render.large-doc-fast", fmt.Sprintf("%.2fs", td), td < 5.0)

	tdir, err := os.MkdirTemp("", "fastnote-st")
	if err != nil {
		check("doc.open", err.Error(), false)
	} else {
		defer os.RemoveAll(tdir)
		doc := filepath.Join(tdir, "n.md")
		content := "原始 内容 — 你好, Привет 🚀\n"
		if err := os.WriteFile(doc, []byte(content), 0o644); err != nil {
			check("doc.open", err.Error(), false)
		} else {
			state := NewAppState(tdir)
			err := actionOpen(state, doc)
			check("doc.open", errMessage(err), err == nil && strings.HasPrefix(state.Doc.Text, "原始"))
			actionInsert(state, "\n尾")
			check("doc.dirty", "", state.Doc.Dirty)
			_, err = actionSave(state)
			check("doc.saved-not-dirty", errMessage(err), err == nil && !state.Doc.Dirty)
			data, _ := os.ReadFile(doc)
			check("doc.roundtrip", "", strings.Contains(string(data), "尾"))

			html := filepath.Join(tdir, "out.html")
			werr := writeHTMLExport(state.Doc.Text, html, "light", "")
			data, _ = os.ReadFile(html)
			s := string(data)
			check("export.html-complete", errMessage(werr),
				werr == nil && strings.Contains(s, "<!DOCTYPE html>") &&
					strings.Contains(s, "<html") && strings.Contains(s, "<style>") &&
					strings.Contains(s, "<title>") && strings.Contains(s, "原始"))

			pdf := filepath.Join(tdir, "out.pdf")
			err = writePDFExport(state.Doc.Text, pdf)
			pdfData, _ := os.ReadFile(pdf)
			check("export.pdf-valid", errMessage(err),
				err == nil && strings.HasPrefix(string(pdfData), "%PDF-1.4") &&
					len(pdfData) > 500)
		}

		os.MkdirAll(filepath.Join(tdir, "sub"), 0o755)
		os.WriteFile(filepath.Join(tdir, "a.md"), []byte("x"), 0o644)

		b2, err := NewFileBrowser("open", tdir)
		check("browser.open-lists-md", errMessage(err),
			err == nil && hasEntry(b2.Entries, "a.md", false))
		if err == nil {
			_, err = b2.Activate("sub")
			check("browser.enter-dir", errMessage(err),
				err == nil && strings.HasSuffix(b2.Cwd, "sub"))
			err = b2.Parent()
			check("browser.parent", errMessage(err),
				err == nil && b2.Cwd == absPath(tdir))
			picked, _ := b2.Activate("a.md")
			check("browser.open-returns-path", "", strings.HasSuffix(picked, "a.md"))
		}
	}

	fb, err := NewFileBrowser("open", "/usr/bin")
	check("browser.lists", errMessage(err),
		err == nil && len(fb.Entries) > 5 && fb.Entries[0].Name == "..")
	if err == nil {
		fb.ShowAll = true
		err = fb.Refresh()
		check("browser.filter-toggle", errMessage(err), err == nil && len(fb.Entries) > 3)
	}

	tdir2, err := os.MkdirTemp("", "fastnote-st2")
	if err == nil {
		defer os.RemoveAll(tdir2)
		src := filepath.Join(tdir2, "s.md")
		os.WriteFile(src, []byte("# CLI"), 0o644)
		st := NewAppState(tdir2)
		err = RunCLIActions(st, src, "\ninserted", true, filepath.Join(tdir2, "o.html"))
		_, statErr := os.Stat(filepath.Join(tdir2, "o.html"))
		check("cli.e2e", errMessage(err),
			err == nil && !st.Doc.Dirty && statErr == nil)
	}

	fmt.Printf("\nselftest: %s\n", selftestVerdict())
	return len(failures) == 0
}

func hasEntry(entries []Entry, name string, wantDir bool) bool {
	for _, e := range entries {
		if e.Name == name && e.IsDir == wantDir {
			return true
		}
	}
	return false
}

func absPath(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

func errMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func selftestVerdict() string {
	if len(failures) == 0 {
		return "all checks passed"
	}
	return fmt.Sprintf("failed: %v", failures)
}