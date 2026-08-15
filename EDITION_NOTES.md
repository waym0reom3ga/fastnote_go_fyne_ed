# FastNote go_fyne Edition — Notes

## Binary size

The go_fyne binary is ~32 MB. This is dominated by Fyne's embedded theme
resources, which are compiled into every Fyne application via `//go:embed`
regardless of application complexity:

| Resource | Size | Purpose |
|----------|------|---------|
| NotoSans-Regular/Bold/Italic/BoldItalic.ttf | ~1.8 MB | UI font (4 variants) |
| EmojiOneColor.otf | 4.1 MB | Emoji rendering |
| DejaVuSansMono-Powerline.ttf | 334 KB | Monospace font |
| InterSymbols-Regular.ttf | 8.8 KB | Symbol glyphs |
| 96 SVG icons + PNG | ~200 KB | Toolbar/theme icons |

The `theme` package is imported **transitively** by `widget`, `container`, and
other Fyne packages — it cannot be excluded without forking Fyne.

### Reducing binary size

Building with the `no_emoji` Go build tag removes `EmojiOneColor.otf` and
reduces the binary by ~4 MB:

```
go build -tags no_emoji -o fastnotes .
```

This has no functional impact on a markdown editor. The remaining ~28 MB is
the NotoSans font family and icon set, which are baked into Fyne's widget
rendering pipeline and cannot be removed without forking the toolkit.

### Comparison

Gio's minimum footprint is ~10.7 MB (single `goregular` font at 149 KB). The
~22 MB difference is attributable to toolkit design choices, not application
complexity: Fyne bundles its own fonts and icons for self-containment; Gio
uses Go's built-in font and the system's OpenGL.
