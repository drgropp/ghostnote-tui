# ghostnote-tui
minimalist terminal note editor — the TUI client of the ghostnote ecosystem.


Command-driven **terminal note editor** — part of the Ghostnote
ecosystem. Pure black background, a configurable accent color, and a `:` command
system inspired by modal editors.

All Ghostnote tools share the same notes, stored as plain JSON in
`~/.ghostnote/notes/` (the `ghostnote/v1` format). A note made here appears in
the Ghostnote CLI and web app, and vice versa.

<img width="1124" height="589" alt="image" src="https://github.com/user-attachments/assets/6566d836-4f79-470e-9fd6-0a16344a4d8b" />


## Install

Requires [Go](https://go.dev/dl/) 1.22+.

```bash
go install github.com/drgropp/ghostnote-tui@latest
```

This builds `ghostnote-tui` onto your Go bin path (`$(go env GOPATH)/bin`,
usually `~/go/bin`). Ensure that directory is on your `PATH`, then run:

```bash
ghostnote-tui
```

### Build from source

```bash
git clone https://github.com/drgropp/ghostnote-tui.git
cd ghostnote-tui
go build -o ghostnote-tui .   # or: go install .
```

## Using it

Start typing — you're in **TYPE** mode. Press `:` to open the command bar, type
a command, press Enter. `Esc` cancels the command bar.

### Note commands

| Command | Description |
|---|---|
| `:save [name]` | Save the current note (uses the current name if omitted) |
| `:open [name]` | Load a saved note (alias `:load`) |
| `:notes` | Open the navigable notes list (aliases `:list`, `:ls`) |
| `:new` | Start a fresh note |
| `:rename [name]` | Rename the current note |
| `:copy [name]` | Save a copy under a new name |
| `:delete [name]` | Delete a note (aliases `:del`, `:rm`) |
| `:search [term]` | Find notes by name **or** contents; results open in the list |
| `:export [name] [path]` | Write a note to a `.ghostnote.json` file |
| `:import [path]` | Import a `.ghostnote.json` file |
| `:wipe` | Clear the editor (alias `:clear`) |
| `:help` | Show the command summary |
| `:quit` | Exit (aliases `:q`; also `Ctrl+C`) |

### Appearance commands

| Command | Description |
|---|---|
| `:ui [color]` | Set **both** the accent and cursor color |
| `:ui accent [color]` | Set just the accent (borders, title, status bar) |
| `:ui cursor [color]` | Set just the cursor |
| `:ui rgb` | Animated full-spectrum rainbow |
| `:ui rgb spectrum` | Smooth fade through red → orange → yellow → green → blue → purple |
| `:ui rgb static` | A fixed vivid color |
| `:reset` | Restore the default Ghost Cyan theme |

Colors accept palette names — `red`, `orange`, `yellow`, `green`, `blue`,
`purple`, `white`, `gray`, `black`, `cyan`/`ghost` — or any hex value like
`#ff00aa`. Your theme persists to `~/.ghostnote/tui-config.json` and is restored
on the next launch.

### The notes list

Open it with `:notes` (or after a `:search`). Then:

- `↑`/`↓` or `j`/`k` — move the selection
- `Enter` — open the highlighted note
- `x` or `d` — delete the highlighted note
- `Esc` — back to the editor

## The note format (`ghostnote/v1`)

Notes are individual JSON files in `~/.ghostnote/notes/<name>.ghostnote.json`,
the same format used across the ecosystem. The TUI edits the `text` field
(plain markdown). If a note has a `drawing` made in the web app's canvas, the
TUI **preserves it** on save — it just doesn't render it in the terminal.

## The Ghostnote ecosystem

- **ghostnote** — the web app (canvas + text, runs in the browser)
- **ghostnote-tui** — this terminal editor
- **ghostnote-cli** — the CLI and local sync daemon

All share the `ghostnote/v1` format and the `~/.ghostnote/notes/` directory.

## Platform support

Built in Go, so it should run on **Windows, macOS, and Linux**. Developed and
tested on Windows; macOS and Linux are expected to work (no platform-specific
code — paths use `os.UserHomeDir()` and `filepath`) but are currently
unverified. Reports from other platforms are welcome.

## Built with

[Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles), and
[Lipgloss](https://github.com/charmbracelet/lipgloss).

## License

MIT
