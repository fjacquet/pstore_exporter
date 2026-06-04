# 1. Record Architecture Decisions

## Status

Accepted

## Context

As the `pstore_exporter` project grows, significant architectural choices are
made that affect how the codebase is structured, what trade-offs were accepted,
and why alternatives were rejected. Without a structured record, this context
lives only in commit messages, PR descriptions, or the memories of contributors
who may not be available later.

## Decision

We will record significant architectural decisions as ADRs in the `docs/adr/`
directory. Each ADR is numbered sequentially, written in MADR-lite format
(`# <number>. <title>`, `## Status`, `## Context`, `## Decision`,
`## Consequences`), and stored as a Markdown file. Once accepted, an ADR is
not deleted — it may be superseded by a later ADR, which will reference it.

## Consequences

- Future contributors can understand *why* a design was chosen, not just *what*
  it is.
- Architectural discussions happen before code is written, reducing costly
  late-stage refactors.
- The ADR index in `docs/adr/README.md` provides a navigable overview.
- There is a small ongoing cost: any significant architectural change requires
  a new or updated ADR as part of the pull request.
