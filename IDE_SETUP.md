# IDE Setup Guide for Go Development

> **Goal:** Get your editor configured for productive Go development in under 10 minutes.

---

## Quick Recommendation

**VS Code + Go Extension** — Best for beginners, free, works everywhere.

| Editor | Best For | Setup Time |
|--------|----------|------------|
| **VS Code** | Beginners, cross-platform | 5 min |
| **GoLand** | Professional development | 10 min (paid) |
| **Neovim** | Terminal enthusiasts | 30 min |

---

## Option 1: VS Code (Recommended)

### Step 1: Install VS Code

Download from: https://code.visualstudio.com

### Step 2: Install Go Extension

1. Open VS Code
2. Press `Ctrl+Shift+X` (or `Cmd+Shift+X` on Mac)
3. Search for "Go"
4. Install the official **Go** extension by Google

### Step 3: Install Go Tools

Open the Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`) and run:

```
Go: Install/Update Tools
```

Select all tools and click OK. This installs:
- `gopls` — Language server (autocomplete, errors, hover docs)
- `gofmt` / `goimports` — Code formatting
- `dlv` — Debugger
- `staticcheck` — Linter
- And more

### Step 4: Configure Settings

Create or edit `.vscode/settings.json` in your project:

```json
{
    "[go]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "golang.go",
        "editor.codeActionsOnSave": {
            "source.organizeImports": "explicit"
        }
    },
    "go.useLanguageServer": true,
    "go.lintOnSave": "workspace",
    "go.vetOnSave": "workspace",
    "go.testFlags": ["-v", "-race"],
    "go.coverOnSingleTest": true,
    "go.coverOnSingleTestFile": true
}
```

### What This Does

| Setting | Effect |
|---------|--------|
| `formatOnSave` | Runs `goimports` on save — auto-formats + manages imports |
| `source.organizeImports` | Adds missing imports, removes unused ones |
| `useLanguageServer` | Enables `gopls` for autocomplete, errors, go-to-definition |
| `lintOnSave` | Catches common mistakes on save |
| `testFlags` | Always run tests with verbose + race detection |
| `coverOnSingleTest` | Shows coverage when running individual tests |

### Useful Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `F12` | Go to definition |
| `Shift+F12` | Find all references |
| `F2` | Rename symbol |
| `Ctrl+Space` | Trigger autocomplete |
| `F5` | Start debugging |
| `Ctrl+Shift+F5` | Restart debugging |
| `F9` | Toggle breakpoint |

### Debugging Setup

Create `.vscode/launch.json`:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch Package",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}",
            "args": ["view", "data.csv"]
        },
        {
            "name": "Launch Current File",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${file}"
        },
        {
            "name": "Run Tests",
            "type": "go",
            "request": "launch",
            "mode": "test",
            "program": "${workspaceFolder}"
        }
    ]
}
```

---

## Option 2: GoLand

### Step 1: Install GoLand

Download from: https://www.jetbrains.com/go/

### Step 2: Configure

GoLand works out of the box. Recommended settings:

1. **Settings → Go → Go Modules** → Enable Go modules integration
2. **Settings → Editor → General → Go Imports** → Enable "Optimize imports on the fly"
3. **Settings → Editor → Code Style → Go** → Use `gofmt` (default)

### Useful Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+B` / `Cmd+B` | Go to definition |
| `Ctrl+F12` / `Cmd+F12` | File structure |
| `Shift+F6` | Rename |
| `Ctrl+Shift+F10` | Run tests |
| `Shift+F9` | Debug |
| `Ctrl+Alt+T` | Surround with (if/for/etc.) |

---

## Option 3: Neovim

### Using nvim-lspconfig

```lua
-- init.lua or plugin config
require('lspconfig').gopls.setup{
    settings = {
        gopls = {
            analyses = {
                unusedparams = true,
            },
            staticcheck = true,
            gofumpt = true,
        },
    },
}
```

### Using lazy.nvim + extras

```lua
-- In your lazy.nvim config
{
    "ray-x/go.nvim",
    dependencies = { "ray-x/guihua.lua" },
    config = function()
        require("go").setup()
    end,
    event = {"CmdlineEnter"},
    ft = {"go", 'gomod'},
}
```

---

## Essential Tools

### Install Globally

```bash
# Language server
go install golang.org/x/tools/gopls@latest

# Formatting + imports
go install golang.org/x/tools/cmd/goimports@latest

# Linter
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Debugger
go install github.com/go-delve/delve/cmd/dlv@latest

# Static analysis
go install honnef.co/go/tools/cmd/staticcheck@latest
```

### Verify Installation

```bash
gopls version
dlv version
staticcheck --version
```

---

## Go Playground (No Setup Required)

For quick experiments without installing anything:

**https://go.dev/play/**

- Share code via URL
- No account needed
- Great for testing snippets

---

## Common Issues

### "gopls not found"

```bash
go install golang.org/x/tools/gopls@latest
# Restart VS Code
```

### "Formatting not working"

1. Check `go env GOPATH` — make sure `$GOPATH/bin` is on PATH
2. Run `Go: Install/Update Tools` from Command Palette
3. Restart VS Code

### "Import not recognized"

Run `go mod tidy` in your terminal:
```bash
cd your-project
go mod tidy
```

### "Race detector not working"

```bash
# Test with race detection
go test -race ./...
```

---

## Next Steps

1. **Open your first Go file** — the extension will activate
2. **Try autocomplete** — type `fmt.` and see suggestions
3. **Try debugging** — set a breakpoint and press F5
4. **Try running tests** — click the "run test" link above a test function

---

> **Tip:** The Go extension shows documentation on hover. Just move your mouse over any function, type, or package name to see what it does. This is the fastest way to learn the standard library.
