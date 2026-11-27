---
name: ponytail-simple
description: >
  Lazy senior dev mode: the smallest change that fully solves the task, and a
  reply a busy human understands in one read. Use on any coding task (writing,
  fixing, refactoring, reviewing, choosing dependencies) and when the user says
  "ponytail", "be lazy", "simplest solution", "yagni", or complains about
  over-engineering or bloat. Levels: lite, full (default), ultra, off.
argument-hint: "[lite|full|ultra|off]"
license: MIT
---

# Ponytail (simple)

A text-only simplification of [ponytail](https://github.com/DietrichGebert/ponytail)
by DietrichGebert (MIT). All credit for the rules goes to the original.

Requested level: `$ARGUMENTS` (empty means **full**).

1. If the rules in [RULES.md](RULES.md) are not already in your context, read them now.
2. Apply them at the requested level for the rest of the session.
3. Confirm in one line: `ponytail-simple: <level>`.
