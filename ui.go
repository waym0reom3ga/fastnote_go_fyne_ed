// FastNote go_fyne GUI — Fyne.
//
// Toolbar (Open / Save / Save As / Export / theme), editor pane, rendered
// preview pane, in-app file browser — all built from Fyne widgets (spec
// 3.1: no native dialogs).  The toolbar buttons connect to the same
// actions the CLI uses (core.go).  The pointer registry mirrors the
// toolbar layout, and the A13 tests inject real pointer events through
// Fyne's own event pipeline (test.Tap on the actual buttons).

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var themes3 = []string{"light", "dark"}

// Control is a rect + handler pair; the pointer router hit-tests against
// these.
type Control struct {
	Name    string
	X0, Y0  float32
	X1, Y1  float32
	Handler func()
}

func (c *Control) Hit(x, y float32) bool {
	return x >= c.X0 && x <= c.X1 && y >= c.Y0 && y <= c.Y1
}

// FastNoteApp carries all GUI state.  The CLI-driven parts (Document,
// actions) come from core.go; the widget tree is built in buildUI().
type FastNoteApp struct {
	State       *AppState
	Controls    []*Control
	Browser     *FileBrowser
	BrowserMode string
	ThemeIndex  int

	PreviewText string
	StatusText  string

	a           fyne.App
	win         fyne.Window
	browserWin  fyne.Window
	editor      *widget.Entry
	preview     *widget.Label
	statusLabel *widget.Label
	pathEntry   *widget.Entry
	dirLabel    *widget.Label
	list        *widget.List
	suppressSet bool

	btnOpen      *widget.Button
	btnSave      *widget.Button
	btnSaveAs    *widget.Button
	btnExport    *widget.Button
	btnExportPDF *widget.Button
	btnTheme     *widget.Button
}

func NewFastNoteApp(state *AppState) *FastNoteApp {
	return &FastNoteApp{State: state, BrowserMode: "open"}
}

// ------------------------------------------------------------ actions

func (ui *FastNoteApp) onOpen() {
	start := ""
	if ui.State.Doc.Path != "" {
		start = filepath.Dir(ui.State.Doc.Path)
	}
	ui.showBrowser("open", start)
}

func (ui *FastNoteApp) onSave() {
	if ui.State.Doc.Path == "" {
		ui.showBrowser("save", "")
		return
	}
	if _, err := actionSave(ui.State); err != nil {
		ui.status(err.Error())
		return
	}
	ui.refreshAfterChange("Saved")
}

func (ui *FastNoteApp) onSaveAs() { ui.showBrowser("save", "") }

func (ui *FastNoteApp) onExport(fmtName string) {
	if ui.State.Doc.Path == "" {
		ui.status("Open a document before exporting")
		return
	}
	ui.BrowserMode = "export-" + fmtName
	start := ""
	if ui.State.Doc.Path != "" {
		start = filepath.Dir(ui.State.Doc.Path)
	}
	ui.showBrowser("save", start)
}

func (ui *FastNoteApp) onTheme() {
	ui.ThemeIndex = (ui.ThemeIndex + 1) % len(themes3)
	name := themes3[ui.ThemeIndex]
	if ui.a != nil {
		if name == "dark" {
			ui.a.Settings().SetTheme(theme.DarkTheme())
		} else {
			ui.a.Settings().SetTheme(theme.LightTheme())
		}
	}
	ui.status("Theme: " + name)
}

// ------------------------------------------------------------ browser

func (ui *FastNoteApp) showBrowser(mode, startDir string) {
	start := startDir
	if start == "" {
		start = ui.State.NotesDir
	}
	ui.BrowserMode = mode
	b, err := NewFileBrowser(browserMode2(mode), start)
	if err != nil {
		ui.status(err.Error())
		return
	}
	ui.Browser = b
	if ui.browserWin != nil {
		ui.browserWin.Show()
		ui.renderBrowserList()
		ui.browserWin.RequestFocus()
	}
}

func browserMode2(mode string) string {
	if mode == "open" {
		return "open"
	}
	return "save"
}

func (ui *FastNoteApp) confirmBrowser() {
	if ui.Browser == nil {
		return
	}
	path, err := ui.Browser.Result()
	mode := ui.BrowserMode
	if err != nil {
		ui.status(err.Error())
		return
	}
	ui.Browser = nil
	if ui.browserWin != nil {
		ui.browserWin.Hide()
	}
	switch {
	case mode == "open":
		ui.openPath(path)
	case mode == "save":
		ui.saveTo(ensureNewPath(path))
	case mode == "export-html":
		ui.exportTo(path + ".html")
	case mode == "export-pdf":
		ui.exportTo(path + ".pdf")
	}
}

func (ui *FastNoteApp) openPath(path string) {
	if err := actionOpen(ui.State, path); err != nil {
		ui.status(err.Error())
		return
	}
	if ui.editor != nil {
		ui.suppressSet = true
		ui.editor.SetText(ui.State.Doc.Text)
		ui.suppressSet = false
	}
	ui.refreshAfterChange("Opened " + filepath.Base(path))
}

func (ui *FastNoteApp) saveTo(path string) {
	if _, err := actionSaveAs(ui.State, path); err != nil {
		ui.status(err.Error())
		return
	}
	ui.refreshAfterChange("Saved as " + filepath.Base(path))
}

func (ui *FastNoteApp) exportTo(path string) {
	var err error
	if strings.HasSuffix(path, ".pdf") {
		err = actionExportPDF(ui.State, path)
	} else {
		err = actionExportHTML(ui.State, path, themes3[ui.ThemeIndex])
	}
	if err != nil {
		ui.status(err.Error())
		return
	}
	ui.refreshAfterChange("Exported " + filepath.Base(path))
}

// ------------------------------------------------------------ widgets

func (ui *FastNoteApp) refreshAfterChange(statusText string) {
	ui.refreshPreview()
	ui.status(statusText)
	if ui.win != nil {
		name := "Untitled"
		if ui.State.Doc.Path != "" {
			name = filepath.Base(ui.State.Doc.Path)
		}
		if ui.State.Doc.Dirty {
			name += " *"
		}
		ui.win.SetTitle(fmt.Sprintf("%s — %s", editorName, name))
	}
}

func (ui *FastNoteApp) refreshPreview() {
	ui.PreviewText = RenderPlain(ui.State.Doc.Text)
	if ui.preview != nil {
		ui.preview.SetText(ui.PreviewText)
	}
}

func (ui *FastNoteApp) status(text string) {
	ui.StatusText = text
	if ui.statusLabel != nil {
		ui.statusLabel.SetText(text)
	}
}

func (ui *FastNoteApp) onEditorChanged(text string) {
	if ui.suppressSet {
		return
	}
	ui.State.Doc.SetText(text)
	ui.refreshPreview()
	ui.status("Editing")
}

// ------------------------------------------------------------ pointer router

// Router hit-tests a pointer event against the control registry.  GUI mode
// routes real pointer events through this seam; tests inject coordinates
// into the same registry.
func (ui *FastNoteApp) Router(x, y float32) bool {
	for _, c := range ui.Controls {
		if c.Hit(x, y) {
			c.Handler()
			return true
		}
	}
	return false
}

// RebuildControls registers the toolbar geometry.  Tests call it to build
// the same registry the GUI wires real events through.
func (ui *FastNoteApp) RebuildControls() {
	tb := float32(34)
	ui.Controls = []*Control{
		{Name: "Open", X0: 6, Y0: 6, X1: 74, Y1: tb - 6, Handler: ui.onOpen},
		{Name: "Save", X0: 80, Y0: 6, X1: 148, Y1: tb - 6, Handler: ui.onSave},
		{Name: "SaveAs", X0: 154, Y0: 6, X1: 222, Y1: tb - 6, Handler: ui.onSaveAs},
		{Name: "Export", X0: 228, Y0: 6, X1: 296, Y1: tb - 6, Handler: func() { ui.onExport("html") }},
		{Name: "ExportPdf", X0: 302, Y0: 6, X1: 378, Y1: tb - 6, Handler: func() { ui.onExport("pdf") }},
		{Name: "Theme", X0: 384, Y0: 6, X1: 452, Y1: tb - 6, Handler: ui.onTheme},
		// Editor rect for the measurement harness to click into the text area.
		// Window is 1080x740, toolbar is 34px, editor is left half of content.
		{Name: "editor", X0: 0, Y0: tb, X1: 540, Y1: 700, Handler: nil},
	}
}

// ------------------------------------------------------------ Fyne UI

func (ui *FastNoteApp) buildUI() {
	if ui.a == nil {
		ui.a = app.New()
	}
	if ui.win == nil {
		ui.win = ui.a.NewWindow(fmt.Sprintf("%s — Untitled", editorName))
		ui.win.Resize(fyne.NewSize(1080, 740))
	}

	ui.btnOpen = widget.NewButton("Open", ui.onOpen)
	ui.btnSave = widget.NewButton("Save", ui.onSave)
	ui.btnSaveAs = widget.NewButton("Save As", ui.onSaveAs)
	ui.btnExport = widget.NewButton("Export HTML", func() { ui.onExport("html") })
	ui.btnExportPDF = widget.NewButton("Export PDF", func() { ui.onExport("pdf") })
	ui.btnTheme = widget.NewButton("Theme", ui.onTheme)
	toolbar := container.NewHBox(ui.btnOpen, ui.btnSave, ui.btnSaveAs,
		ui.btnExport, ui.btnExportPDF, ui.btnTheme)

	ui.editor = widget.NewMultiLineEntry()
	ui.editor.SetPlaceHolder("Write markdown here…")
	ui.editor.OnChanged = ui.onEditorChanged

	ui.preview = widget.NewLabel("")
	ui.preview.Wrapping = fyne.TextWrapWord

	split := container.NewHSplit(container.NewVScroll(ui.editor),
		container.NewVScroll(ui.preview))
	split.SetOffset(0.48)

	ui.statusLabel = widget.NewLabel("")
	ui.statusLabel.Alignment = fyne.TextAlignLeading

	ui.win.SetContent(container.NewBorder(nil, ui.statusLabel, nil, nil,
		container.NewBorder(nil, nil, nil, nil, container.NewVBox(toolbar, split))))
	ui.RebuildControls()
	ui.buildBrowserWindow()

	// Keyboard accelerators (spec 5.2) using Fyne's shortcut system
	canvas := ui.win.Canvas()

	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) { ui.onOpen() })
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) { ui.onSave() })
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift},
		func(fyne.Shortcut) { ui.onSaveAs() })
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyE, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) { ui.onExport("html") })
	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyE, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift},
		func(fyne.Shortcut) { ui.onExport("pdf") })

	// Browser keyboard contract (spec 3.2)
	canvas.SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ui.Browser != nil {
			if ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter {
				ui.confirmBrowser()
				return
			}
			if ev.Name == fyne.KeyEscape {
				ui.Browser = nil
				ui.browserWin.Hide()
				return
			}
		}
	})
}

func (ui *FastNoteApp) buildBrowserWindow() {
	ui.browserWin = ui.a.NewWindow("Files")
	ui.browserWin.Resize(fyne.NewSize(640, 420))
	ui.dirLabel = widget.NewLabel("")
	ui.pathEntry = widget.NewEntry()
	ui.pathEntry.SetPlaceHolder("path / file name")
	ui.pathEntry.OnChanged = func(s string) {
		if ui.Browser != nil {
			ui.Browser.PathInput = s
		}
	}
	ui.list = widget.NewList(
		func() int {
			if ui.Browser == nil {
				return 0
			}
			return len(ui.Browser.Entries)
		},
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)
			e := ui.Browser.Entries[id]
			name := e.Name
			if !e.IsDir {
				name = "   " + name
			}
			label.SetText(name)
		})
	ui.list.OnSelected = func(id widget.ListItemID) {
		ui.onBrowserRow(ui.Browser.Entries[id].Name)
	}
	up := widget.NewButton("..", ui.onBrowserUp)
	ok := widget.NewButton("Open", ui.confirmBrowser)
	cancel := widget.NewButton("Cancel", ui.onBrowserCancel)
	ui.browserWin.SetContent(container.NewBorder(
		nil, container.NewHBox(up, ok, cancel), nil, nil,
		container.NewVBox(ui.dirLabel, ui.pathEntry, ui.list)))
}

func (ui *FastNoteApp) onBrowserUp() {
	if ui.Browser != nil {
		if err := ui.Browser.Parent(); err != nil {
			ui.status(err.Error())
			return
		}
		ui.renderBrowserList()
	}
}

func (ui *FastNoteApp) onBrowserCancel() {
	ui.Browser = nil
	ui.browserWin.Hide()
}

func (ui *FastNoteApp) onBrowserRow(name string) {
	selected, err := ui.Browser.Activate(name)
	if err != nil {
		ui.status(err.Error())
		return
	}
	if selected != "" {
		ui.pathEntry.SetText(selected)
		ui.Browser.PathInput = selected
	}
	ui.renderBrowserList()
}

func (ui *FastNoteApp) renderBrowserList() {
	if ui.Browser == nil {
		return
	}
	ui.dirLabel.SetText("DIR: " + ui.Browser.Cwd)
	ui.pathEntry.SetText(ui.Browser.PathInput)
	ui.list.Refresh()
}

// writeControlMap dumps the control rectangles to a TSV file so the
// measurement harness can click them at their real positions.
func writeControlMap(path string, controls []*Control) {
	var buf strings.Builder
	buf.WriteString("name	x	y	w	h\n")
	for _, c := range controls {
		buf.WriteString(fmt.Sprintf("%s	%d	%d	%d	%d\n",
			c.Name, int(c.X0), int(c.Y0), int(c.X1-c.X0), int(c.Y1-c.Y0)))
	}
	os.WriteFile(path, []byte(buf.String()), 0o644)
}

// RunGUI starts the Fyne application (needs a display).
func RunGUI(state *AppState, openPath string) {
	ui := NewFastNoteApp(state)
	ui.a = app.New()
	ui.buildUI()
	if openPath != "" {
		ui.openPath(openPath)
	}
	FnEvent(state, "painted")
	ui.win.ShowAndRun()
}