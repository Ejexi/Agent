# tools/implementations/subagent/

Subagent tools — spawn and resume sub-agent sessions.

## Tools

| Tool       | Description                                                  |
| ---------- | ------------------------------------------------------------ |
| `subagent` | Spawns a new sub-agent session with a system prompt and task |
| `resume`   | Resumes an existing sub-agent session with additional input  |

## Registration

Registered in bootstrap as:

- `subagent_tool.NewSubagentTool(tracker)`
- `subagent_tool.NewResumeTool(tracker)`
