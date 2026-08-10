// FastNote export writers — HTML (FR-7) and PDF (FR-8).

package main

import (
	"os"
	"path/filepath"
	"strings"
)

func writeHTMLExport(text, path, theme, customCSS string) error {
	page := RenderPage(text, docTitle(text), theme, customCSS)
	return os.WriteFile(path, []byte(page), 0o644)
}

func writePDFExport(text, path string) error {
	plain := RenderPlain(text)
	return os.WriteFile(path, PDFFromLines(plain, 11), 0o644)
}

// ensureNewPath enforces an extension when the user typed a bare path.
func ensureNewPath(path string) string {
	if filepath.Ext(path) == "" {
		return path + ".md"
	}
	return path
}

func exportForExt(fmtName string) string {
	if strings.EqualFold(fmtName, "pdf") {
		return ".pdf"
	}
	return ".html"
}