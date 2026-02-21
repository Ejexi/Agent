---
trigger: always_on
---

# Architecture Definition

This system uses Hexagonal Architecture.

Layers:

domain
kernel
ports
adapters
tools
cmd

Dependency Rules:

domain → no dependencies

kernel → domain only

ports → domain only

adapters → ports only

tools → ports + domain

cmd → kernel only

Forbidden:

domain → adapters

kernel → adapters

tools → infrastructure directly