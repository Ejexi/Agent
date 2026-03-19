# DuckOps TUI — User Guide

## Starting

```bash
duckops          # launches TUI (default)
duckops --scan   # launches conversational scan REPL
duckops --cli    # launches legacy LLM REPL
```

---

## Chat Interface

### Slash Commands `/`

Type `/` at the start of an empty input to open the command menu. Use `↑↓` to navigate, `Enter` to select.

| Command | Action |
|---------|--------|
| `/scan` | Run a full security scan on the current project |
| `/vuln` | Show all known security findings |
| `/status` | Docker + session health check |
| `/help` | Show this reference |
| `/tools` | List all available agent tools |
| `/skills` | List all loaded knowledge skills |
| `/clear` | Clear the conversation history |
| `/logout` | Quit DuckOps |

### File Mentions `@`

Type `@` followed by a filename to attach its content to your message. The file content is injected as a code block before the message is sent to the agent.

```
> review @internal/auth/handler.go for security issues
> explain what @go.mod contains
> what does @docker-compose.yml expose to the network?
```

- Press **Tab** to autocomplete from the fuzzy file search
- Works anywhere in your message, not just at the start
- Files over 100KB are noted but not inlined
- Supports relative and absolute paths

### Shell Commands `!`

Type `!` followed by a shell command to run it directly in a PTY.

```
!ls -la
!git log --oneline -10
!docker ps -a
!cat go.mod
```

Press **Tab** to autocomplete from indexed PATH commands.

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Alt+Enter` | Insert newline (multi-line input) |
| `Ctrl+C` | Quit / interrupt running process |
| `Ctrl+B` | Toggle side panel |
| `Ctrl+K` | Show keyboard shortcuts popup |
| `↑ / ↓` | Scroll conversation |
| `Tab` | Autocomplete `@file` or `!command` |
| `Esc` | Close menu / popup |

---

## Side Panel

Press **Ctrl+B** to toggle the side panel. It shows:

- Active model name and token usage
- Live file tree of the current working directory
  - Directories shown first
  - File icons by language/type
  - 2 levels deep (noise directories hidden)

---

## Paste Support

The input field grows up to 12 lines. Large pastes are handled via bracketed paste — the full content is accepted without truncation (up to 32,000 characters).

---

## Natural Language Scan (--scan mode)

```bash
duckops --scan
```

Launches a conversational scan agent. Examples:

```
> scan this project
> check for hardcoded secrets only
> scan ./src for critical issues
> look for vulnerabilities in ./backend
> is docker ready?
> download scanner images
```

The agent automatically selects the right scanners based on your project's language and framework stack.

---

## Output

All output is rendered via [glamour](https://github.com/charmbracelet/glamour) — Markdown with ANSI colors, adapting automatically to your terminal's light/dark theme.

Scan reports include:

1. **Orchestrator Analysis** — LLM-generated system understanding + risk overview
2. **Summary table** — findings by severity
3. **Per-category status** — which scanners ran and what they found
4. **Full findings** — grouped by severity with location and remediation
