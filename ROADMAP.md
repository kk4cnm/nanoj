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

## Planned (priority order)

### 1. Search filters
Extend the existing `^W` search with a small query grammar so large documents
are navigable by predicate, not just substring:

- `key:name` — match on key
- `type:number` — match by JSON type
- `value:123` — match on value
- (stretch) `key=value` — key/value equality

**Approach:** parse the query into predicates and feed them through the existing
`rowMatches` / `cellMatches` path in both views. Plain text (no prefix) keeps
working as today. Low risk, no architectural change. Good first contribution.

### 2. Schema awareness (`--schema schema.json`)
The headline feature for nanoj's audience (MCP manifests, OpenAPI, Home
Assistant, Kubernetes). A JSON Schema becomes a read-only overlay:

- invalid nodes highlighted
- required fields marked
- `enum` values offered as a pick-list (reuses the existing `modeChoice` UI)
- expected type shown/enforced on edit
- field `description` shown in the status/help line

**Approach (phased):**
- **A.** Validate the whole document with a library
  (`santhosh-tekuri/jsonschema`, draft 2020-12); map its error locations
  (JSON pointers) to nodes and highlight. Add required-field markers and
  `description` display. All read-only.
- **B.** `enum` pick-lists and type-expected hints during editing.
- **C.** The hard parts — `$ref`, `allOf`/`anyOf`/`oneOf`, conditionals.
  Support last, and accept imperfect coverage.

**Traps:** full JSON Schema is a large spec; lean on the library for *matching*
and keep our own logic to "find the subschema for this path" (for enum/
description/required). Don't promise full-spec support.

### 3. JSON diff (`--diff a.json b.json`, and compare-against-disk)
Structured diffs beat line diffs for JSON. Render a read-only overlay with
`+` / `-` / `~` markers by path.

**Approach:** tree-diff keyed by path — objects compared by key, arrays
**positionally** for v1. Reuse the path-overlay pattern from schema.

**Trap:** array move/LCS detection is a rabbit hole — explicitly out of scope
for v1.

### 4. Read-only inspection mode (`--view`)
Browse without any chance of mutation: gate the editing keys, show a
`[read-only]` indicator. Framed honestly as a **safety/intent** mode — note it
does *not* meaningfully reduce memory (undo snapshots are only created on edit,
so a browsing session never accrues them anyway). Cheap; could ride along with
the diff work.

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
