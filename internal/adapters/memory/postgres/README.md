# adapters/memory/postgres/

PostgreSQL implementation of `ports.MemoryPort`.

## Purpose

Key-value memory storage backed by PostgreSQL. Used as a fallback for simple storage needs.

> Prefer purpose-specific adapters (`metadata/postgres/`, `vectordb/pgvector/`) for new code.
