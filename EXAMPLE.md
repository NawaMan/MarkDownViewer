# EXAMPLE.md — a Markdown feature tour

A single document that exercises most of what `viewmd` renders: GitHub-flavoured
Markdown through [Marked], with heading anchors, wrapped tables and a copy button
on every code block. Use it as a smoke test after changing the viewer's CSS or
rendering pipeline.

> **Tip** — every heading below has a generated anchor, so
> [jump straight to the tables](#tables) and the URL will carry the fragment.

---

## Headings

# Heading level 1
## Heading level 2
### Heading level 3
#### Heading level 4
##### Heading level 5
###### Heading level 6

Levels 1 and 2 get an underline rule, matching GitHub. Everything below level 4
falls back to bold body text at roughly paragraph size.

## Text styling

This paragraph is deliberately long so you can watch how the content column
behaves as the sidebar is dragged wider and narrower. It contains *italic*,
**bold**, ***bold italic***, ~~struck through~~, `inline code`, and a stretch of
plain prose to give the line-height something to work with.

Inline variations worth eyeballing: `code_with_underscores`, `--flags`, a
`really-long-inline-code-token-that-should-not-break-the-layout-even-slightly`,
and a mix of **bold containing `code`** plus *italics containing [a link](#links)*.

Unicode passes straight through: ✅ ⚠️ → ≈ ° µ — “curly quotes” and em-dashes.

Line breaks are *not* automatic — `breaks` is off, so this sentence
and this one join into a single paragraph. End a line with a backslash\
to force a break, like that.

## Lists

Unordered, nested three deep:

- Top level item
- Another item, this one with a longer body so it wraps onto a second line when
  the sidebar is wide and the content column is correspondingly narrow
  - Second level
  - Second level with `code`
    - Third level
    - Third level, **bold**
- Back to the top

Ordered, including a non-1 start and a nested block:

1. First step
2. Second step

   A second paragraph inside list item two. Indented four spaces so it stays
   attached to the item rather than closing the list.

3. Third step
   1. Sub-step one
   2. Sub-step two

Task lists:

- [x] Ship the folder icons
- [x] Make folders collapsible
- [ ] Decide whether local image files should be served
- [ ] Wire up a dark-mode toggle
  - [x] Nested, done
  - [ ] Nested, pending

Loose vs. tight — this list has blank lines between items, so each item becomes
its own paragraph:

- Loose item one

- Loose item two

## Links

- Inline: [the project README](README.md) and [agent notes](AGENTS.md)
- Inline with title: [Marked](https://marked.js.org "The Markdown parser used here")
- Reference style: [CommonMark spec][spec]
- Bare autolink: <https://github.com/NawaMan/MarkDownViewer>
- Anchor within this page: [back to the top](#examplemd--a-markdown-feature-tour)
- Relative link to a sibling document: [notes](./README.md)

Clicking a `.md` link loads it through the viewer rather than leaving the page.

## Images

![The viewmd pipeline: a folder of Markdown, one binary, a rendered browser view][diagram]

The diagram above is an inline SVG **data URI**, so it renders with no network
and no extra files on disk. Below is a remote image, which needs internet access:

![Build status badge](https://img.shields.io/badge/status-early-blue)

An image can also be wrapped in a link — click through to the repository:

[![Repository badge](https://img.shields.io/badge/GitHub-MarkDownViewer-black)](https://github.com/NawaMan/MarkDownViewer)

> **Note on local image files** — `viewmd` serves Markdown only. A relative
> reference such as `![diagram](images/flow.png)` will **not** load, because
> there is no route that returns image files from the served folder. Use a data
> URI or an absolute URL until that changes.

## Code

Short, with a language tag:

```go
func main() {
	root := flag.String("folder", ".", "directory to scan")
	flag.Parse()
	if err := serve(*root); err != nil {
		log.Fatal(err)
	}
}
```

Shell, including a line long enough to scroll sideways:

```bash
./booth exec --run --keep-alive -- bash -lc 'cd /home/coder/code && ./viewmd --folder /tmp/sample --md README.md --port 987 --bind 0.0.0.0 --daemon'
```

JSON, to check punctuation-heavy highlighting and the copy button:

```json
{
  "folder": "/home/coder/code",
  "initialMd": "EXAMPLE.md",
  "port": 987,
  "tree": { "files": ["README.md"], "dirs": { "docs": { "files": [] } } }
}
```

A fenced block with **no** language, which should stay unstyled:

```
$ viewmd --status
viewmd: running on port 8765 (pid 4703)
```

Indented code blocks work too:

    four spaces of indent
    still the same block

## Blockquotes

> A single-line quote.

> A quote that runs to a second paragraph.
>
> The second paragraph, with **bold** and `code` inside it.

> Nesting:
>
> > One level deeper.
> >
> > > And a third, which is about as far as anyone should go.

> Quotes can hold other blocks:
>
> - a list item
> - another item
>
> ```go
> fmt.Println("even code")
> ```

## Tables

Default alignment:

| File | Purpose | Lines |
|------|---------|-------|
| `main.go` | Flag parsing and entry point | 340 |
| `server.go` | HTTP routes and handlers | 180 |
| `tree.go` | Directory walk, Markdown filter | 165 |

Explicit alignment — left, centre, right:

| Flag | Default | Meaning |
|:-----|:-------:|--------:|
| `--folder` | `.` | Directory to scan |
| `--port` | `8765` | Listen port |
| `--bind` | `0.0.0.0` | Listen address |
| `--daemon` | *off* | Serve in the background |

A wide table, to confirm it scrolls inside its own box instead of pushing the
page sideways:

| Target | GOOS | GOARCH | Binary | CGO | Notes |
|--------|------|--------|--------|-----|-------|
| Linux x86-64 | `linux` | `amd64` | `viewmd-linux-amd64` | disabled | Runs on musl and glibc alike |
| Linux ARM64 | `linux` | `arm64` | `viewmd-linux-arm64` | disabled | Raspberry Pi, Graviton |
| macOS Intel | `darwin` | `amd64` | `viewmd-darwin-amd64` | disabled | Unsigned, expect Gatekeeper |
| macOS Apple silicon | `darwin` | `arm64` | `viewmd-darwin-arm64` | disabled | Unsigned, expect Gatekeeper |
| Windows x86-64 | `windows` | `amd64` | `viewmd-windows-amd64.exe` | disabled | Installs to `%LOCALAPPDATA%` |
| Windows ARM64 | `windows` | `arm64` | `viewmd-windows-arm64.exe` | disabled | Tested on a native runner |

Cells can hold formatting, including `code`, **bold**, and [links](README.md):

| Input | Renders as |
|-------|-----------|
| `**bold**` | **bold** |
| `` `code` `` | `code` |
| `[link](#tables)` | [link](#tables) |
| `a \| b` | a \| b |

## Horizontal rules

Three ways to write one, all identical once rendered:

---

***

___

## Inline HTML

Markdown passes raw HTML through, so a collapsible block works:

<details>
<summary>Click to expand the long version</summary>

Hidden until you open it — handy for FAQ entries and long logs. Markdown inside
still renders:

- a list
- with `code`

</details>

Keyboard keys via `<kbd>`: press <kbd>Shift</kbd> + <kbd>Wheel</kbd> to scroll
the sidebar sideways.

Sub and superscript: H<sub>2</sub>O, and E = mc<sup>2</sup>.

<p align="center"><strong>A centred paragraph, via inline HTML.</strong></p>

## Escaping

Literal characters that would otherwise be markup: \*not italic\*,
\_not emphasis\_, \# not a heading, \`not code\`, and a backslash \\ itself.

Ampersands and angle brackets survive: AT&T, 5 < 6 > 4, `<div>` in code.

## A long tail

The rest of this section exists so the page scrolls far enough to check that the
sidebar stays fixed, the heading anchors resolve, and returning to a document
restores the right scroll position.

### Subsection one

Content under a level-three heading, with a [link back to the top](#examplemd--a-markdown-feature-tour).

### Subsection two

More content. Nothing surprising here — the point is vertical distance.

### Subsection three

The last one. If you made it here and everything above rendered cleanly, the
viewer is in good shape.

[Marked]: https://marked.js.org
[spec]: https://spec.commonmark.org/
[diagram]: data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI1MjAiIGhlaWdodD0iMTUwIiB2aWV3Qm94PSIwIDAgNTIwIDE1MCIgcm9sZT0iaW1nIiBhcmlhLWxhYmVsPSJ2aWV3bWQgcGlwZWxpbmUgZGlhZ3JhbSI+CjxyZWN0IHdpZHRoPSI1MjAiIGhlaWdodD0iMTUwIiByeD0iMTAiIGZpbGw9IiNmNmY4ZmEiLz4KPGcgZm9udC1mYW1pbHk9Ii1hcHBsZS1zeXN0ZW0sU2Vnb2UgVUksSGVsdmV0aWNhLEFyaWFsLHNhbnMtc2VyaWYiIGZvbnQtc2l6ZT0iMTMiIHRleHQtYW5jaG9yPSJtaWRkbGUiPgo8cmVjdCB4PSIxOCIgeT0iNDUiIHdpZHRoPSIxMjAiIGhlaWdodD0iNTgiIHJ4PSI4IiBmaWxsPSIjZGRmNGZmIiBzdHJva2U9IiMwOTY5ZGEiIHN0cm9rZS13aWR0aD0iMS41Ii8+Cjx0ZXh0IHg9Ijc4IiB5PSI3MCIgZmlsbD0iIzA5NjlkYSIgZm9udC13ZWlnaHQ9IjYwMCI+Zm9sZGVyPC90ZXh0Pgo8dGV4dCB4PSI3OCIgeT0iODgiIGZpbGw9IiM1OTYzNmUiPioubWQ8L3RleHQ+CjxyZWN0IHg9IjIwMCIgeT0iNDUiIHdpZHRoPSIxMjAiIGhlaWdodD0iNTgiIHJ4PSI4IiBmaWxsPSIjZmZmOGM1IiBzdHJva2U9IiNiZjg3MDAiIHN0cm9rZS13aWR0aD0iMS41Ii8+Cjx0ZXh0IHg9IjI2MCIgeT0iNzAiIGZpbGw9IiM3ZDRlMDAiIGZvbnQtd2VpZ2h0PSI2MDAiPnZpZXdtZDwvdGV4dD4KPHRleHQgeD0iMjYwIiB5PSI4OCIgZmlsbD0iIzU5NjM2ZSI+b25lIGJpbmFyeTwvdGV4dD4KPHJlY3QgeD0iMzgyIiB5PSI0NSIgd2lkdGg9IjEyMCIgaGVpZ2h0PSI1OCIgcng9IjgiIGZpbGw9IiNkYWZiZTEiIHN0cm9rZT0iIzFhN2YzNyIgc3Ryb2tlLXdpZHRoPSIxLjUiLz4KPHRleHQgeD0iNDQyIiB5PSI3MCIgZmlsbD0iIzFhN2YzNyIgZm9udC13ZWlnaHQ9IjYwMCI+YnJvd3NlcjwvdGV4dD4KPHRleHQgeD0iNDQyIiB5PSI4OCIgZmlsbD0iIzU5NjM2ZSI+cmVuZGVyZWQ8L3RleHQ+CjxwYXRoIGQ9Ik0xNDIgNzRoNTAiIHN0cm9rZT0iIzU5NjM2ZSIgc3Ryb2tlLXdpZHRoPSIxLjUiIG1hcmtlci1lbmQ9InVybCgjYSkiLz4KPHBhdGggZD0iTTMyNCA3NGg1MCIgc3Ryb2tlPSIjNTk2MzZlIiBzdHJva2Utd2lkdGg9IjEuNSIgbWFya2VyLWVuZD0idXJsKCNhKSIvPgo8dGV4dCB4PSIyNjAiIHk9IjEzMCIgZmlsbD0iIzU5NjM2ZSIgZm9udC1zaXplPSIxMSI+c2NhbiAmIzg1OTQ7IHNlcnZlICYjODU5NDsgcmVuZGVyPC90ZXh0Pgo8L2c+CjxkZWZzPjxtYXJrZXIgaWQ9ImEiIG1hcmtlcldpZHRoPSI3IiBtYXJrZXJIZWlnaHQ9IjciIHJlZlg9IjYiIHJlZlk9IjMiIG9yaWVudD0iYXV0byI+PHBhdGggZD0iTTAgMGw2IDMtNiAzeiIgZmlsbD0iIzU5NjM2ZSIvPjwvbWFya2VyPjwvZGVmcz4KPC9zdmc+Cg==
