# nanoj Roadmap

This is a living sketch of where nanoj is headed, in rough priority order. It
records not just *what* but the intended *approach* and the known traps, so
contributors can pick something up without re-deriving the design. Nothing here
is a commitment to a timeline.

The guiding constraint for everything below: **valid by construction**. The
document model holds exactly the six JSON types, the only path to disk is the
strict serializer, and views are *renderings* of one source-of-truth tree (see
[docs/DESIGN.md](docs/DESIGN.md)). New features should preserve that, not erode
it.

A recurring implementation pattern worth naming: **read-only path overlays.**
Both schema awareness and diff annotate nodes by their structural path
(`"/docs/0/title"`-style, the same keys the tree/table already use for
expansion and the table array). The annotation layer never mutates the tree; it
just decorates rows during render. This keeps powerful features cheap and
non-invasive.

## Shipped

- v0.1 tree editor; v0.2 table view + accessibility theming + large-file speed;
  v0.3 table search, inline cell edit, cut/copy/paste, sort-by-column; v0.4
  `--lenient` JSONC input.
- **Search filters** — `^W` predicate grammar (`key:`/`type:`/`value:`/`key=value`),
  ANDed, in both views; plain substring still works.
- **Schema awareness** (`--schema`) — read-only overlay via
  `santhosh-tekuri/jsonschema`: invalid values marked (`✗`), required and
  missing-required fields, expected type + `description` in the status line,
  `enum` pick-lists (reusing `modeChoice`), and `$ref`/`$defs` resolved by
  walking the compiled schema. Combinators are surfaced per-field where
  unambiguous: `allOf` branches are flattened into the working set (their
  properties/required/types/descriptions all apply), while `anyOf`/`oneOf`
  contribute alternative types and merged enums for display only — we never
  descend into their properties, since which branch applies depends on the data.
- **JSON diff** (`--diff baseline.json working.json`) — path-keyed structural
  overlay: `+`/`~` markers inline and a `[diff +A~C-R]` summary; objects by key,
  arrays positional. Removed nodes have no working-tree path, so they surface as
  the `-R` count rather than inline rows.
- **Overlay markers in the table view** — schema and diff render in both views:
  each table column reserves two trailing cells for a marker, a left gutter
  carries row-level annotations (missing required key, added row), Enter on an
  enum-constrained cell opens the pick-list, and the status line shows the
  selected cell's schema/diff info.
- **Read-only mode** (`--view`) — gates every edit and save; `[read-only]` badge;
  navigation/search/expand still work. Composes with `--schema` and `--diff`.
- **YAML / TOML import** (`--from yaml|toml`) — scoped as explicit one-way
  **conversion**, per the direction below: `internal/convert` decodes into the
  same six-type tree (key order preserved via `yaml.Node` / TOML metadata, big
  ints intact, anchors expanded, datetimes → RFC 3339 strings, `inf`/`nan`
  rejected at load), and the editor opens a **JSON working copy** (`config.yaml`
  → buffer at `config.json`, `[from yaml]` badge). The source file is never
  written back, so the valid-by-construction JSON promise is untouched.

Schema, diff and read-only share the **read-only path overlay** pattern (below):
they annotate nodes by structural path during render and never mutate the tree.

## Planned

Nothing scheduled — the original roadmap is fully shipped. Known candidates,
none committed:

- **YAML/TOML export** — the write half of interop. If ever done it must be a
  deliberate, clearly-labeled action *outside* the editing path (the only save
  path stays the strict JSON serializer); e.g. a separate convert-and-exit mode
  rather than a save target.
- **Schema `allOf`/`anyOf`/`oneOf` branch pinning** — per-field combinators are
  surfaced (see above), but we don't report *which* `oneOf` branch matched the
  current data.

## Non-goals (for now)
- Full JSON5 input (single-quoted strings, unquoted keys). The available Go
  JSON5 libraries decode to maps and lose key order, which we preserve. `--lenient`
  already covers the common cases (comments, trailing commas).
- Comment-preserving round-trip editing — it would fight the six-type model.
- A windowed tree flatten — profiling showed rendering is already viewport-bound;
  see the note in [docs/DESIGN.md](docs/DESIGN.md).
