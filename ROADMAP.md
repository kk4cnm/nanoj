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

The last three share the **read-only path overlay** pattern (below): they
annotate nodes by structural path during render and never mutate the tree.

## Planned (priority order)

### 5. YAML / TOML interop (deferred, scoped as conversion)
High adoption potential but the only item that touches nanoj's identity. Reading
is easy (`yaml.v3` / `BurntSushi/toml` → the same tree); the hard question is
what to *write*. The valid-by-construction promise is a JSON promise, and YAML
round-tripping (comments, anchors, tags) is lossy.

**Direction if/when we do it:** treat it as explicit **conversion**, not magic —
e.g. `nanoj --from yaml config.yaml` opens a JSON working copy, and export is a
deliberate, clearly-labeled choice. Keep it out of the core editing path so the
value proposition stays crisp.

## Non-goals (for now)
- Full JSON5 input (single-quoted strings, unquoted keys). The available Go
  JSON5 libraries decode to maps and lose key order, which we preserve. `--lenient`
  already covers the common cases (comments, trailing commas).
- Comment-preserving round-trip editing — it would fight the six-type model.
- A windowed tree flatten — profiling showed rendering is already viewport-bound;
  see the note in [docs/DESIGN.md](docs/DESIGN.md).
