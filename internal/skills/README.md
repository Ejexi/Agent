# Skills & Capabilities (`agent/internal/skills`)

## Purpose
The `skills` directory contains the definition and embedded data for specialized "Skills" that the DuckOps Agent can dynamically load into its context. Skills act as foundational knowledge bases or specific behavioral markdown guides.

## Subdirectories & Files
- `data/`: Likely contains the raw markdown files or static assets defining individual skills.
- `embedded.go`: Logic (often using Go's `//go:embed`) to bundle markdown skill definitions into the compiled binary so they are always available.

## Architectural Rules
- Skills represent declarative knowledge and instructions rather than imperative code. They should be treated as static context to enhance the LLM's understanding of specific tasks (e.g., how to do specific types of security scans).
