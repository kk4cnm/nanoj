# nanoj

A terminal app for viewing and editing JSON — think nano, but it understands
JSON structure so you can't accidentally break the formatting.

nanoj is for anyone who hand-edits JSON and is tired of hunting down a missing
comma or an unbalanced brace. You navigate and edit a structured view of your
data; nanoj writes the file back out as clean, valid JSON every time. Bad
formatting isn't *caught* — it's impossible, because you never edit the raw
text directly.

> **Status:** early development. A working interactive tree editor — navigate,
> edit values, change types, add/delete nodes, and save — backed by a tested
> valid-by-construction JSON engine. See [docs/DESIGN.md](docs/DESIGN.md).

## Why it exists

JSON is everywhere in modern LLM/agent tooling — MCP servers, tool
definitions, config. Hand-editing it is error-prone, and a single formatting
slip wastes time. nanoj makes well-formed JSON the only possible output.

## Try the current build

Requires [Go](https://go.dev/dl/) 1.21+.

```sh
go build -o nanoj .
./nanoj path/to/file.json
```

Keys (nano-style; `^X` means Ctrl-X):

| Key | Action |
| --- | --- |
| `↑`/`↓`, `Ctrl-P`/`Ctrl-N` | move |
| `→` / `←` | expand / collapse (or descend / jump to parent) |
| `Enter` | edit a value, toggle a bool, or expand/collapse a container |
| `t` | change a value's type |
| `a` | add a key (object) or element (array) |
| `^K` / `M-6` / `^U` | cut / copy / paste a node (recoverable via paste or undo) |
| `^W` | search keys and values (reveals matches in collapsed branches) |
| `^T` | toggle the table view for an array of objects |
| `M-U` / `M-E` | undo / redo (also `^Z` / `^Y`) |
| `^O` | write the file |
| `^X` | exit (prompts if there are unsaved changes) |

### Table view

When a document is an array of objects, nanoj opens it as a spreadsheet-style
grid (rows = elements, columns = the union of keys). You can also press `^T` on
any such array in the tree view. Arrow keys move between cells and `Enter`
edits a cell: a string/number keeps its type, a bool toggles, and a blank or
null cell is filled with a value whose type is inferred from what you type
(`42` → number, `true` → bool, empty → null, otherwise text). Nested values are
edited in the tree view. `^T` or `Esc` returns to the tree.

Wide tables scroll horizontally as you move: the row-number column stays fixed,
and `‹`/`›` markers in the status bar show when more columns lie off-screen.
`^A`/`^E` jump to the first/last column, `Home`/`End` to the first/last row, and
`PgUp`/`PgDn` page through rows. `^W` searches cell values and jumps to the next
match (scrolling it into view). `^K`/`M-6`/`^U` cut/copy/paste whole rows —
handy for duplicating a row (copy then paste).

## Configuration

nanoj reads an optional JSON config file (so you can edit it in nanoj itself).
Write a starter file to the default location with:

```sh
nanoj --write-config
```

The file is looked up at `--config <path>`, then `$NANOJ_CONFIG`, then your
per-OS config dir (e.g. `~/.config/nanoj/config.json` on Linux,
`~/Library/Application Support/nanoj/config.json` on macOS).

Options:

| Field | Values | Meaning |
| --- | --- | --- |
| `defaultView` | `auto` (default), `tree`, `table` | which view to open in (`auto` = table for arrays of objects) |
| `indent` | a string, e.g. `"  "` or `"\t"` | indentation used when saving |
| `theme` | `default`, `colorblind`, `high-contrast`, `mono` | base color/attribute palette |
| `styles` | per-element overrides | fine-tune individual elements |

### Accessibility

Theming is built around accessibility, not just looks:

- **`colorblind`** — an Okabe–Ito-based palette chosen to stay distinguishable
  across common forms of color blindness, and it reinforces type differences
  with **bold**/*italic* so meaning never depends on hue alone.
- **`high-contrast`** — bright, bold colors for low-vision use.
- **`mono`** — no color at all; element types are told apart purely by bold,
  italic, underline, and faint.
- The standard **`NO_COLOR`** environment variable (and `NANOJ_NO_COLOR`) is
  honored — when set, nanoj drops all color and falls back to attribute-only
  styling.

Each element (`key`, `string`, `number`, `bool`, `null`, `structure`, `header`,
`selection`) can be individually overridden with `fg`, `bg`, `bold`, `italic`,
`underline`, `faint`, and `reverse`:

```json
{
  "theme": "colorblind",
  "styles": {
    "string": { "fg": "#3cb371", "italic": true },
    "key":    { "fg": "12", "bold": true }
  }
}
```

## Design

nanoj is written in Go for single static binaries across macOS, Windows,
Linux, and the BSDs, with a low barrier for contributors. The full reasoning —
the "valid by construction" principle, why the tree is the source of truth,
and why we lean on the standard-library JSON parser — is in
[docs/DESIGN.md](docs/DESIGN.md).

## License

[MIT](LICENSE)
