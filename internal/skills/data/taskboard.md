# Task Management: `duckops board`

Use task board for planning, tracking progress, documenting work, resuming work, and collaborating with other agents.

## Quick Reference

```bash
duckops board whoami                             # Current agent identity
duckops board create agent                       # Create a new agent identity (requried for task assignment)

duckops board list boards                        # List boards
duckops board list cards <board_id> [--status todo|in-progress|pending-review|done]
duckops board get <id>                           # Get board/card details (auto-detects type)
duckops board mine [--status <status>]           # Your assigned cards

duckops board create board "Name" --description "Desc"
duckops board create card <board_id> "Task" --description "Details"
duckops board create checklist <card_id> --item "Step 1" --item "Step 2"
duckops board create comment <card_id> "Note"

duckops board update card <card_id> --status in-progress --assign-to-me
duckops board update card <card_id> --add-tag urgent --remove-tag blocked
duckops board update checklist-item <item_id> --check|--uncheck
```

**Formats:** `--format table` (default), `json`, `simple` (IDs only)
**Statuses:** `todo`, `in-progress`, `pending-review`, `done` (use hyphens)

## Workflow

```bash
# 1. Create & plan
duckops board create agent # to get agent id
duckops board create card <board_id> "Feature X" --description "Requirements"
duckops board create checklist <card_id> --item "Research" --item "Implement" --item "Validate"
export AGENT_BOARD_AGENT_ID=<agent_id> && duckops board update card <card_id> --status in-progress --assign-to-me

# 2. Track progress (comments = working memory)
duckops board create comment <card_id> "Found: API needs X-Token header"
duckops board update checklist-item <item_id> --check

# 3. Complete
duckops board update card <card_id> --status done

# 4. Blocked?
duckops board update card <card_id> --add-tag blocked --add-tag needs-human
duckops board create comment <card_id> "BLOCKED: need AWS creds"
```

## Rules

- One in-progress card at a time
- Comments = memory (findings, decisions, gotchas)
- Only mark done when FULLY complete
- Check `duckops board mine` at session start

## When to Use

Multi-step tasks, complex implementations, cross-session work. Skip for simple Q&A.

**For complex multi-phase tasks** (migrations, large implementations, multi-week projects):

1. Do your research first
2. Present plan and verify with user
3. Create board with cards to track execution - don't just leave it as markdown
