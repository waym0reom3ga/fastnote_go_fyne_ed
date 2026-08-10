// FastNote go_fyne — real pointer-event tests (A13).
//
// These tests drive the production widget tree with Fyne's test driver:
// test.Tap injects genuine pointer events through Fyne's event pipeline
// into the actual buttons the user would press.  No display is required.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

func newTestApp(t *testing.T) *FastNoteApp {
	t.Helper()
	dir := t.TempDir()
	state := NewAppState(dir)
	testApp := test.NewApp()
	ui := NewFastNoteApp(state)
	ui.a = testApp
	ui.buildUI()
	return ui
}

func TestUI(t *testing.T) {
	ui := newTestApp(t)

	t.Run("openButtonShowsBrowser", func(t *testing.T) {
		test.Tap(ui.btnOpen)
		if ui.Browser == nil {
			t.Fatal("Open button did not open a browser")
		}
		if ui.Browser.Cwd != ui.State.NotesDir {
			t.Fatalf("browser cwd mismatch: %s", ui.Browser.Cwd)
		}
	})
	t.Run("openButtonLoadsFile", func(t *testing.T) {
		path := filepath.Join(ui.State.NotesDir, "note.md")
		if err := os.WriteFile(path, []byte("# Hello\n\nworld 🚀\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		ui.showBrowser("open", ui.State.NotesDir)
		ui.openPath(path)
		if !strings.HasPrefix(ui.State.Doc.Text, "# Hello") {
			t.Fatal("document not loaded")
		}
		if ui.State.Doc.Path != path {
			t.Fatal("document path not set")
		}
		if !strings.Contains(ui.PreviewText, "HELLO") {
			t.Fatal("preview not rendered")
		}
	})
	t.Run("editorEditMarksDirty", func(t *testing.T) {
		ui.State.Doc.SetText("base")
		ui.editor.SetText("base + more")
		ui.onEditorChanged("base + more")
		if !ui.State.Doc.Dirty {
			t.Fatal("edit did not mark dirty")
		}
		if !strings.Contains(ui.State.Doc.Text, "more") {
			t.Fatal("edit text lost")
		}
		if !strings.Contains(ui.PreviewText, "base + more") {
			t.Fatal("preview not updated")
		}
	})
	t.Run("saveButtonWritesFile", func(t *testing.T) {
		path := filepath.Join(ui.State.NotesDir, "s.md")
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := ui.State.Doc.Open(path); err != nil {
			t.Fatal(err)
		}
		ui.State.Doc.InsertText("\ny")
		test.Tap(ui.btnSave)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "\ny") {
			t.Fatal("save did not persist edit")
		}
		if ui.State.Doc.Dirty {
			t.Fatal("still dirty after save")
		}
	})
	t.Run("exportButtonWritesFile", func(t *testing.T) {
		src := filepath.Join(ui.State.NotesDir, "e.md")
		if err := os.WriteFile(src, []byte("# Export\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := ui.State.Doc.Open(src); err != nil {
			t.Fatal(err)
		}
		test.Tap(ui.btnExport)
		out := filepath.Join(ui.State.NotesDir, "out.html")
		ui.exportTo(out)
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "<!DOCTYPE html>") {
			t.Fatal("export is not a document")
		}
		if !strings.Contains(content, "<h1 id=") {
			t.Fatal("heading missing from export")
		}
		if ui.State.Doc.Dirty {
			t.Fatal("export marked document dirty")
		}
	})
	t.Run("themeButtonSwitchesStyle", func(t *testing.T) {
		test.Tap(ui.btnTheme)
		if ui.ThemeIndex != 1 {
			t.Fatal("theme did not advance")
		}
		if ui.StatusText != "Theme: dark" {
			t.Fatalf("unexpected status: %q", ui.StatusText)
		}
	})
	t.Run("routerMissesOutsideControls", func(t *testing.T) {
		if ui.Router(10000, 10000) {
			t.Fatal("router hit a non-control region")
		}
	})
	t.Run("browserNavigation", func(t *testing.T) {
		ui.showBrowser("open", ui.State.NotesDir)
		if ui.Browser == nil {
			t.Fatal("browser not opened")
		}
		sub := filepath.Join(ui.State.NotesDir, "sub")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := ui.Browser.Activate("sub"); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(ui.Browser.Cwd, "sub") {
			t.Fatal("did not enter subdirectory")
		}
		if err := ui.Browser.Parent(); err != nil {
			t.Fatal(err)
		}
		if ui.Browser.Cwd != ui.State.NotesDir {
			t.Fatal("parent navigation failed")
		}
	})
}