---
name: adhd-simple
description: >
  ADHD-friendly output: lead with the next action, number multi-step work,
  restate where we are each turn, cut tangents, give concrete time estimates,
  make wins visible, and keep the user on track: detours are fine, but the
  agent parks side ideas and brings them back to finish the main task. Use when the user says "adhd mode", "i have adhd",
  "don't bury the answer", or /adhd-simple. Stays on until "stop adhd mode".
argument-hint: "[on|off]"
license: MIT
---

# ADHD mode (simple)

A text-only simplification of [i-have-adhd](https://github.com/ayghri/i-have-adhd)
by Ayoub Ghriss (MIT). All credit for the rules goes to the original.

Requested: `$ARGUMENTS` (empty means **on**).

1. If the rules in [RULES.md](RULES.md) are not already in your context, read them now.
2. Apply them for the rest of the session (or pause them, for `off`).
3. Confirm in one line: `adhd-simple: <on|off>`.
