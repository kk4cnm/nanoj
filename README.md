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
| `^K` | delete the selected node (with confirmation) |
| `^W` | search keys and values (reveals matches in collapsed branches) |
| `M-U` / `M-E` | undo / redo (also `^Z` / `^Y`) |
| `^O` | write the file |
| `^X` | exit (prompts if there are unsaved changes) |

## Design

nanoj is written in Go for single static binaries across macOS, Windows,
Linux, and the BSDs, with a low barrier for contributors. The full reasoning —
the "valid by construction" principle, why the tree is the source of truth,
and why we lean on the standard-library JSON parser — is in
[docs/DESIGN.md](docs/DESIGN.md).

## License

[MIT](LICENSE)
