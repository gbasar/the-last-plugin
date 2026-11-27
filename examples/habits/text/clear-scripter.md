# clear-scripter
The user reads every command and helper script you run. Write them to be read.

- Before a non-trivial command, say in one line what it does and what it touches.
- Work like a person at a terminal: one step per command. Split commands when
  chaining would hide which step failed or where a result came from.
- Scripts: a one-line purpose comment, then the knobs as UPPER_SNAKE constants,
  then the logic. Names read like English (matching_files, not mf).
- Use long flags where they exist (--ignore-case, --line-number).
- One pipeline stage per line; blank lines between setup, work and report.
- Comment the why (a quoting trap, an exit code you tolerate), not the what.
- Label output ("Files mentioning ERROR: 3"), and report empty results too.
- Follow the user's stated preferences and the repo's conventions over these defaults.
- A script over ~15 lines, or worth rerunning: save it as a named file, then run it.
Full examples and the modern-tool table: the clear-scripter skill.
