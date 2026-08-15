// FastNote go_gio — document model and shared action layer.
//
// The actions in this file are the ONLY place the application's behaviour is
// implemented.  The GUI callbacks and the headless CLI both call these same
// functions (specification 5.2, shared-path rule), so a button cannot rot
// while the CLI still works.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	editorName = "FastNote"
	version    = "1.0.0"
	portID     = "go_fyne"
)

var appExtensions = []string{".md", ".markdown", ".txt"}

// NoteError is a user-visible failure (open/save/export).  Carried as an
// error value so the GUI and CLI treat failures identically.
type NoteError struct{ msg string }

func (e *NoteError) Error() string { return e.msg }

func noteErrf(format string, a ...any) error {
	return &NoteError{msg: fmt.Sprintf(format, a...)}
}

// Document is an in-memory markdown document with dirty tracking.
type Document struct {
	Path  string
	Text  string
	Dirty bool
}

func (d *Document) SetText(text string) {
	if text != d.Text {
		d.Text = text
		d.Dirty = true
	}
}

func (d *Document) Open(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return noteErrf("cannot open %s: %v", path, stripErr(err))
	}
	d.Path = path
	d.Text = string(data)
	d.Dirty = false
	return nil
}

// InsertText appends text (the --insert seam drives FR-3 through this path).
func (d *Document) InsertText(text string) {
	d.Text += text
	d.Dirty = true
}

func (d *Document) Save() (string, error) {
	if d.Path == "" {
		return "", noteErrf("no file name: use save-as (FR-6)")
	}
	if err := d.write(d.Path); err != nil {
		return "", err
	}
	return d.Path, nil
}

func (d *Document) SaveAs(path string) (string, error) {
	if err := d.write(path); err != nil {
		return "", err
	}
	d.Path = path
	return path, nil
}

func (d *Document) write(path string) error {
	if err := os.WriteFile(path, []byte(d.Text), 0o644); err != nil {
		return noteErrf("cannot save %s: %v", path, stripErr(err))
	}
	d.Dirty = false
	return nil
}

// AppState is application state shared between GUI and CLI.
type AppState struct {
	Doc       *Document
	NotesDir  string
	SavedOnce bool
	EventFile string // path to event file for phase markers (spec 5.1)
}

func NewAppState(notesDir string) *AppState {
	if notesDir == "" {
		notesDir, _ = os.UserHomeDir()
	}
	abs, err := filepath.Abs(notesDir)
	if err != nil {
		abs = notesDir
	}
	return &AppState{Doc: &Document{}, NotesDir: abs}
}

func stripErr(err error) string {
	if err == nil {
		return ""
	}
	if os.IsNotExist(err) {
		return "no such file"
	}
	if os.IsPermission(err) {
		return "permission denied"
	}
	return err.Error()
}

// ------------------------------------------------------------ event markers

// FnEvent appends a phase marker to the event file (spec 5.1).
// A reporting outlet only — never drives or simulates an operation.
func FnEvent(state *AppState, marker string) {
	if state == nil || state.EventFile == "" {
		return
	}
	f, err := os.OpenFile(state.EventFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, marker)
}

// ------------------------------------------------------------ actions

func actionOpen(state *AppState, path string) error {
	if err := state.Doc.Open(path); err != nil {
		return err
	}
	FnEvent(state, "open")
	return nil
}

func actionInsert(state *AppState, text string) { state.Doc.InsertText(text) }

func actionSave(state *AppState) (string, error) {
	path, err := state.Doc.Save()
	if err == nil {
		state.SavedOnce = true
		FnEvent(state, "save")
	}
	return path, err
}

func actionSaveAs(state *AppState, path string) (string, error) {
	path, err := state.Doc.SaveAs(path)
	if err == nil {
		state.SavedOnce = true
		FnEvent(state, "save-as")
	}
	return path, err
}

func actionExportHTML(state *AppState, path, theme string) error {
	if err := writeHTMLExport(state.Doc.Text, path, theme, ""); err != nil {
		return err
	}
	FnEvent(state, "export-html")
	return nil
}

func actionExportPDF(state *AppState, path string) error {
	if err := writePDFExport(state.Doc.Text, path); err != nil {
		return err
	}
	FnEvent(state, "export-pdf")
	return nil
}

// RunCLIActions executes the headless seam in the mandated order (spec 5.1).
func RunCLIActions(state *AppState, openPath, insert string, doSave bool, exportPath string) error {
	var err error
	if openPath != "" {
		if err = actionOpen(state, openPath); err != nil {
			return err
		}
	}
	if insert != "" {
		actionInsert(state, insert)
	}
	if doSave {
		if _, err = actionSave(state); err != nil {
			return err
		}
	}
	if exportPath != "" {
		if strings.HasSuffix(strings.ToLower(exportPath), ".pdf") {
			err = actionExportPDF(state, exportPath)
		} else {
			err = actionExportHTML(state, exportPath, "light")
		}
	}
	return err
}

// ErrNotFound wraps os errors into NoteError for browser listings.
func listErr(dir string, err error) error {
	return noteErrf("cannot list %s: %v", dir, stripErr(err))
}

var _ = errors.Is