# Subagents for Parallel Work

Delegate parallelizable tasks to subagents to save context and increase throughput. Subagent results are not visible to the user—summarize key findings in your response.

**RECURSIVE SUBAGENTS:**
You can grant the `dynamic_subagent_task` tool to a subagent you spawn. This allows that subagent to divide its work further. 
- Maximum Depth: 3 (Root Agent -> Subagent -> Child -> Grandchild).
- Recommended: Only grant the subagent tool if the task is broad (e.g., "Scan all 50 repositories in this org").

## When to Use Subagents

- **Parallel exploration**: Multiple directories, modules, or sources to analyze simultaneously
- **Iterative research**: Multi-round searches, doc lookups, comparing options
- **Open-ended searches**: When you're not confident you'll find the match quickly

## When NOT to Use Subagents

- **Sequential dependencies**: When step N needs output from step N-1
- **Simple lookups**: One file read, one doc search, known file paths—just do it directly
- **Known patterns**: Searching for a specific class/function name—use grep/glob directly

## Parallel Execution

Launch multiple independent subagents in a single message:

```
[
  subagent: "Analyze frontend architecture in /src/web",
  subagent: "Analyze backend API in /src/api",
  subagent: "Review infra setup in /terraform"
]
```

## Tool Selection (Critical)

Subagents require explicit tool lists. **Apply principle of least-privilege**—only grant tools the task actually needs.

**Read-only tools** (safe for research):
`view`, `search_docs`, `view_web_page`, `search_memory`, `load_skill`

**Mutating tools** (grant sparingly):
`create`, `str_replace`, `remove`, `run_command`, `run_command_task`

**Tool selection by task type:**
| Task | Tools | Sandbox? |
|------|-------|----------|
| Codebase exploration | `view` | No |
| Doc/web research | `view`, `search_docs`, `load_skill`, `view_web_page` | No |
| Write code | `view`, `create`, `str_replace`, `remove` | Optional |
| Write + validate | `view`, `str_replace`, `run_command` | Optional |
| Run diagnostics / discovery | `view`, `run_command` | Recommended |

**`run_command` in subagents:**

- Use `view` instead of shell commands like `cat`, `ls`, `find` — it's faster and doesn't need approval
- Consider if `view` with grep/glob args can replace the command before reaching for `run_command`
- Constrain commands in the subagent prompt (e.g., "Only run read-only commands" or "Only run `terraform validate`")

**Sandbox mode behavior — understand the tradeoff:**

- **Sandboxed** (`enable_sandbox=true`): Subagent runs AUTONOMOUSLY to completion — no approval pauses. Best when you need many commands to run without user interaction (e.g., parallel discovery, bulk diagnostics). Requires Docker, adds ~5-10s startup overhead.
- **Non-sandboxed** (default): Subagent pauses on each mutating tool call (`run_command`, `create`, `str_replace`, `remove`) waiting for your approval before continuing. Best when you want visibility/control over each step (e.g., applying changes, deploying). Read-only tools never pause regardless.

**Choose based on the situation:**

- Many parallel read-only commands (discovery, diagnostics) → sandbox for autonomy
- A few targeted commands where you want user oversight → no sandbox, let it pause for approval
- Mutating operations (file writes, deploys) → either works: sandbox for speed, no sandbox for control

## Writing Effective Prompts

Subagents have no prior context. Make prompts self-contained:

**Bad:** "Find where the error is handled"

**Good:** "Search /src/services for error handling patterns (try/catch, middleware, exception handlers). Return: 1) files with error handling, 2) main approach used, 3) coverage gaps."

**Always include:**

- Specific paths or file patterns to search
- Expected output format (list, summary, table)
- Whether to research only OR write code

## Example

```
User: "How does auth work in this app?"

# Parallel subagents with tools: [view]
1. "Find auth files in /src—login, session, JWT patterns. List files + purposes."
2. "Find auth middleware and route guards. Document the flow."
3. "Find auth config and related env vars."

# Synthesize results into cohesive answer
```
