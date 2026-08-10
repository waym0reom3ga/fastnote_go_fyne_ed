// Minimal, self-contained PDF writer.
//
// Produces a valid single- or multi-page PDF/1.4 with Helvetica text,
// enough for FastNote's export requirement (FR-8): a file on disk that is
// a valid PDF.  No external library, so no missing-tool failure mode; the
// bytes are written by this port and only this port.

package main

import (
	"strconv"
	"strings"
)

func escPDF(text string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)")
	return replacer.Replace(text)
}

// toLatin1 encodes runes lossily (unknown runes become '?'), mirroring the
// reference port's encode("latin-1", "replace").
func toLatin1(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 256 {
			out = append(out, byte(r))
		} else {
			out = append(out, '?')
		}
	}
	return out
}

func ftoa(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}

func pad10(n int) string {
	s := itoa(n)
	for len(s) < 10 {
		s = "0" + s
	}
	return s
}

// PDFFromLines builds a PDF document from a plain-text rendering (no
// layout engine).
func PDFFromLines(text string, fontPt int) []byte {
	if fontPt <= 0 {
		fontPt = 11
	}
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, l := range rawLines {
		if len(l) > 96 {
			l = l[:96]
		}
		if strings.TrimSpace(l) == "" {
			l = " "
		}
		lines = append(lines, l)
	}
	if len(lines) == 0 {
		lines = []string{" "}
	}

	pageHeight, pageWidth := 842.0, 595.0
	lineH := float64(fontPt) * 1.32
	margin := 56.0
	usable := pageHeight - 2*margin

	pages := [][]string{}
	current := []string{}
	used := 0.0
	for _, line := range lines {
		if used+lineH > usable {
			pages = append(pages, current)
			current = []string{}
			used = 0.0
		}
		current = append(current, line)
		used += lineH
	}
	pages = append(pages, current)

	// content streams
	objects := [][]byte{}
	for _, page := range pages {
		streamLines := []string{}
		y := pageHeight - margin
		for _, line := range page {
			streamLines = append(streamLines,
				"BT /F1 "+itoa(fontPt)+" Tf "+ftoa(margin)+" "+ftoa(y)+
					" Td ("+escPDF(line)+") Tj ET")
			y -= lineH
		}
		content := toLatin1(strings.Join(streamLines, "\n"))
		objects = append(objects, append([]byte("stream\n"), append(content, []byte("\nendstream")...)...))
	}
	if len(objects) == 0 {
		objects = append(objects, []byte("stream\nBT /F1 11 Tf 56 786 Td ( ) Tj ET\nendstream"))
		pages = append(pages, []string{" "})
	}

	out := [][]byte{[]byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")}
	offsets := []int{}
	tell := func() int {
		n := 0
		for _, b := range out {
			n += len(b)
		}
		return n
	}
	// catalog 1
	offsets = append(offsets, tell())
	out = append(out, []byte("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n"))
	// pages tree 2
	kids := ""
	for i := range pages {
		kids += itoa(3+i) + " 0 R "
	}
	offsets = append(offsets, tell())
	out = append(out, []byte("2 0 obj\n<< /Type /Pages /Kids ["+kids+"] /Count "+
		itoa(len(pages))+" >>\nendobj\n"))
	fontObj := 3 + len(pages)
	// each page 3.., and its content stream right after it
	for i := range pages {
		obj := 3 + i
		offsets = append(offsets, tell())
		out = append(out, []byte(itoa(obj)+" 0 obj\n<< /Type /Page /Parent 2 0 R "+
			"/MediaBox [0 0 "+ftoa(pageWidth)+" "+ftoa(pageHeight)+"] "+
			"/Resources << /Font << /F1 "+itoa(fontObj)+" 0 R >> >> "+
			"/Contents "+itoa(obj+len(pages))+" 0 R >>\nendobj\n"))
		offsets = append(offsets, tell())
		out = append(out, []byte(itoa(obj+len(pages))+" 0 obj\n<< /Length "+
			itoa(len(objects[i]))+" >>\n"))
		out = append(out, objects[i])
		out = append(out, []byte("\nendobj\n"))
	}
	offsets = append(offsets, tell())
	out = append(out, []byte(itoa(fontObj)+" 0 obj\n<< /Type /Font /Subtype /Type1 "+
		"/BaseFont /Helvetica >>\nendobj\n"))
	xref := tell()
	out = append(out, []byte("xref\n0 "+itoa(fontObj+1)+"\n"))
	out = append(out, []byte("0000000000 65535 f \n"))
	for _, off := range offsets {
		out = append(out, []byte(pad10(off)+" 00000 n \n"))
	}
	out = append(out, []byte("trailer\n<< /Size "+itoa(fontObj+1)+" /Root 1 0 R >>\n"+
		"startxref\n"+itoa(xref)+"\n%%EOF\n"))

	total := 0
	for _, b := range out {
		total += len(b)
	}
	blob := make([]byte, 0, total)
	for _, b := range out {
		blob = append(blob, b...)
	}
	return blob
}