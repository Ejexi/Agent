# Graphical User Interface (`agent/internal/gui`)

## Purpose
The `gui` directory encompasses the user interface code. For the DuckOps Agent, this primarily involves Terminal-based User Interfaces (TUI).

## Subdirectories
- `tui/`: Contains the interactive terminal user interface implementations (likely built with Bubble Tea/Lipgloss), handling rendering, user input, chat bubbles, and state management for the visual agent.
- `setup/`: Specialized TUI flows for initial application configuration and onboarding.

## Architectural Rules
- **View Layer:** This is an inbound adapter. It strictly handles presentation and input mapping.
- **No Direct Domain Mutations:** The GUI should invoke use cases or trigger events in the `application` layer; it should not directly implement business logic or modify domain objects directly.
