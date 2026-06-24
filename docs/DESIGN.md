# nanoj — Design & Rationale

This document records *why* nanoj is built the way it is, so future
contributors understand the reasoning behind the choices rather than just the
code. It will grow as the project does.

## What nanoj is

nanoj is a terminal application for viewing and editing JSON, with nano-style
hotkeys shown along the bottom of the screen. Its goal is to let humans
create and manipulate JSON — especially the JSON that flows in and out of
LLM tools and MCP servers — **without ever introducing formatting errors**.

## The core principle: valid by construction

The user never edits raw JSON text. They navigate and edit a typed in-memory
tree (`internal/document`), and the only way back out to disk is through a
serializer that walks that tree and emits well-formed JSON.

This means malformed output is not "prevented" by validation after the fact —
it is **structurally impossible**, because there is no code path that lets a
stray comma or unbalanced brace exist. Bad JSON can only enter at *parse*
time, where it is rejected up front (`TestRejectMalformed`).

## The tree is the source of truth; views are renderings

JSON is a tree, not a table. The data model is therefore a tree of typed
nodes. Every way of looking at the data — the keyboard-driven tree view
(Phase 1), and a spreadsheet-style table view for tabular data (Phase 2) — is
just a *rendering* of that one model. No view ever owns or mutates JSON text
directly. This keeps the door open to multiple views without ever
compromising the single source of truth.

## Why Go

The project needs to run everywhere with minimal friction for both users and
contributors. Go fits that better than the alternatives we considered:

- **Single static binaries, trivial cross-compilation.** `GOOS`/`GOARCH` let
  one machine build native binaries for macOS (Intel + Apple Silicon),
  Windows, Linux, and the BSDs, with no runtime for users to install. This is
  the single biggest reason for the choice.
- **Low contribution barrier.** Go is deliberately small and readable, so
  casual contributors can follow a PR. This matters for an open project that
  hopes to attract outside improvement.
- **Mature TUI ecosystem.** The Charm libraries — Bubble Tea (app framework),
  Bubbles (widgets), Lip Gloss (styling) — are the modern standard for Go
  terminal UIs.
- **JSON in the standard library**, including a streaming decoder for large
  files.

The trade-off we accepted: Rust would be faster and more memory-efficient on
very large files, but it raises the contribution barrier and complicates the
build. Go is more than fast enough for the multi-megabyte files in scope, and
the data-model design would not need to change if performance ever forced a
rewrite of a hot path.

## Why we do not write our own JSON parser

`encoding/json` is battle-tested. Using it for parsing means nanoj inherits
correct handling of escaping, Unicode, and number formats for free. Our
correctness story lives on the *output* side (valid by construction), which is
far easier to test exhaustively than parser conformance.

This principle also shaped how lenient (JSONC) input works. The optional
`--lenient` flag tolerates `//` and `/* */` comments and trailing commas, but
rather than writing a JSON5 grammar, nanoj runs a small *string-aware*
preprocessor (`StripJSONC`) that removes those constructs and then hands clean
JSON to the same strict `Parse`. So key order, number precision, and escaping
all flow through the one trusted path. Comments have nowhere to live in the
six-type model, so they are dropped and the user is warned — lenient mode is an
"import and normalize" tool, not comment-preserving round-trip editing (which
would fight the valid-by-construction model). Full JSON5 — single-quoted
strings, unquoted keys — is intentionally out of scope for now, partly because
the available Go JSON5 libraries decode to maps and would lose key order.

## Fidelity choices

- **Key order is preserved.** Objects store members as an ordered slice, not a
  Go map (maps randomize iteration order, which would churn users' files).
- **Numbers preserve their original text** via `json.Number`, so large
  integers and high-precision floats round-trip exactly instead of being
  forced through `float64`.
- **HTML escaping is disabled** on output, so `<`, `>` and `&` are written
  literally and the JSON stays human-readable.

## Roadmap

- **Phase 1 (done):** the tree view — load, keyboard navigation,
  edit values in place, change value type, add/delete nodes, search (^W),
  undo/redo, and save via a nano-style prompt (^O). Files load fully into
  memory. Editing goes through the document package's mutation primitives, so
  the valid-by-construction guarantee holds for every save.

  Two notable design choices here:
  - **Expansion is tracked by structural path** (e.g. `/1/0`), not by node
    pointer. Paths survive the deep-cloning used for undo/redo and belong to
    the view rather than the shared model — so a future table view can ignore
    them entirely.
  - **Undo/redo is snapshot-based:** each mutation deep-clones the tree onto a
    history stack. This is simpler and harder to get wrong than recording
    inverse operations, and documents in scope are small enough that whole-tree
    snapshots are cheap.
- **Phase 2 (in progress):** the table view is implemented — an array of
  objects renders as a grid (rows = elements, columns = the union of keys),
  auto-detected at the document root and reachable via ^T on any tabular array.
  It is a second *rendering* of the same tree: cell edits go through the same
  mutation primitives and undo history as the tree view, and structural changes
  are deferred to the tree view to keep the grid focused on fast value entry.
  Configuration is implemented: an optional JSON config file (`internal/config`)
  sets the default view, indent, and theme, with per-element style overrides.
  Theming is built for accessibility — a colorblind-safe (Okabe–Ito) palette, a
  high-contrast palette, and a monochrome palette that distinguishes element
  types by text attributes alone, plus support for the standard `NO_COLOR`
  environment variable. The config package is deliberately UI-free (pure data);
  the ui package's `BuildTheme` turns that data into lipgloss styles, so colors
  stay fully customizable without coupling config to the terminal layer.

  Wide tables scroll horizontally with a frozen row-number column and
  off-screen-column indicators; all lines are clipped (ANSI-aware) to the
  terminal width so nothing wraps.

  Large files are handled by doing as little work as possible:
  - **Rendering is windowed** in both views — only the rows in the viewport are
    drawn, so file size doesn't affect per-frame cost.
  - **The tree flatten is lazy**: a document that opens in the table view never
    materializes the (potentially huge) flat row list. It is built on demand
    the first time you switch to the tree view. On a ~10 MB array of records
    this cut open time from ~150 ms to ~10 ms.
  - **Table column widths are cached and sampled.** They were previously
    recomputed by scanning every row on every frame (≈200 ms per keystroke on a
    60k-row table); they are now computed once (from a bounded sample of rows)
    and only recomputed when the data changes — ≈0.2 ms per keystroke.

  Known future work: a windowed (lazy) tree flatten so the tree view of a very
  large, fully-expanded document doesn't materialize all rows at once, and a
  smaller per-node memory footprint (the tree currently uses roughly 25–30× the
  source file size in memory).
- **Later:** undo/redo, search and replace, virtualized rendering for very
  large files, optional JSON5/JSONC support.
