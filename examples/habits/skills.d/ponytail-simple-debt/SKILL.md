---
name: ponytail-simple-debt
description: >
  List every `shortcut:` comment (and older `ponytail:` ones) as a debt
  ledger. Report only, changes nothing. Use for "ponytail debt", "what did
  ponytail defer", "list the shortcuts", /ponytail-simple-debt.
argument-hint: "[marker word, e.g. TODO]"
license: MIT
---

<!-- From ponytail-debt in ponytail by DietrichGebert
     (https://github.com/DietrichGebert/ponytail), MIT. -->

Every deliberate shortcut is marked with a `shortcut:` comment naming its
ceiling and upgrade path (older code may say `ponytail:`). Collect them into
one ledger so a deferral can't quietly become permanent.

## Scan

Grep for comment markers, skipping `.git` and everything `.gitignore` ignores:

`git grep --no-index --exclude-standard -nE '(#|//|/[*]) ?(shortcut|ponytail):'`

Add other comment prefixes if the stack uses them. If the user names their own
marker word (`/ponytail-simple-debt TODO`), grep for that word instead.

Each hit is one row. Skip hits that are not a deferral, like a note about a
keyboard shortcut.

## Output

One row per marker, grouped by file:

`<file>:<line>, <what was simplified>. ceiling: <the limit named>. upgrade: <the trigger to revisit>.`

Tag any marker that names no upgrade path or trigger `no-trigger`: those are
the ones that silently rot. Want an owner per row? Add `git blame -L<line>,<line>`.

End with `<N> markers, <M> with no trigger.` Nothing found:
`No shortcut debt. Clean ledger.`

Report only. If asked, write the ledger to a file (e.g. `SHORTCUT-DEBT.md`).
