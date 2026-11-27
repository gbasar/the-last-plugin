---
name: ponytail-simple-review
description: >
  Quality review of a change, or an audit of a whole repo or folder: is the
  logic right, is it safe, does it hold under real load, is risky code tested,
  is it fast enough, and is every line needed. Each finding is explained in
  plain English. Report only, changes nothing. Use for "review this", "code
  review", "review my PR", "is this over-engineered", "audit this codebase",
  "find bloat", "what can I delete", /ponytail-simple-review.
argument-hint: "[change | repo | <path> | <PR>]"
license: MIT
---

<!-- Merges ponytail-review and ponytail-audit from ponytail by DietrichGebert
     (https://github.com/DietrichGebert/ponytail), MIT. -->

Review like the senior developer who will be paged when it breaks.
Order of importance: correct, safe, holds under load, tested, fast, lean.
Lean still matters: every extra line must be read, tested and fixed later.
This is a report the user asked for, so give it in full.

## 0. Pick the scope

- **Change** (default): what the user names (uncommitted or staged changes, a
  branch, a PR, files). Nothing named: the uncommitted changes, or the last
  commit if there are none.
- **Repo**: when the user asks for an audit, or names `repo`, a folder or a
  package. Nothing named: the whole repo.

## 1. Understand first

- **Change:** read the diff, then the code it touches: callers of every
  changed function, the functions it calls, the tests, the README. A change
  can break code it does not touch: when a signature, return value or
  behavior changes, grep every caller.
- **Repo:** read the README, the deploy and build config, the dependency
  list, the entry points (main, routes, handlers, jobs, CLI commands) and the
  tests. In a big repo, go deep where a mistake costs the most, not file by
  file, and say which parts you did not read.
- Both: trace the real flow (where data comes in, what is stored, what goes
  out), reading fully the paths with user input, money, auth, data writes,
  background jobs and anything shared between processes.
- Find the expected load (README, deploy config): one person running a
  script, or many users and processes at once. Judge scale against that, and
  say which load you assumed.

## 2. Look for

1. **Bug:** wrong result, crash, missed edge case (empty, zero, last item,
   rounding, time zones), a caller broken by the change or disagreeing with
   what a function returns, a fix applied in one caller while the shared
   function stays broken, the same rule applied differently in two places.
2. **Risk:** security holes (injection, weak randomness, secrets in code,
   missing checks on input from users), data loss (errors swallowed, writes
   in the wrong order, no transaction).
3. **Scale:** fine for one user, wrong for many: check-then-write races, the
   same work done by every process, memory or lists that only grow, a query
   per item, O(n^2) on big input, per-process state that must be shared.
4. **Missing test:** risky logic (a branch, a parser, money, security, data
   writes, a bug fix) with no test that fails when it breaks. One good test,
   not coverage.
5. **Speed:** big slowdowns are problems. Small wins (work repeated in a hot
   loop) are suggestions; some software counts every millisecond.
6. **Lean:** code that should not exist or should be smaller.
   - delete: dead code, unused options, flags and config, speculative features
   - reuse: the repo already has this helper (name the path)
   - stdlib / native: the standard library or platform already does it; a
     dependency for what a few lines can do
   - yagni: an interface with one implementation, a factory with one product,
     a wrapper that only passes calls through, config nobody sets
   - merge: near-copies that must change together
   - split: one function or class doing several unrelated jobs, so it is hard
     to read or test. Split by job, never by line count, and never into
     helpers that exist only to make a function shorter.

## 3. Check before you report

- Every finding needs a concrete case: "this input or situation leads to this
  wrong result". No case, no finding.
- Re-read the lines and confirm: the caller exists, the value can really be
  empty. Before calling code unused, grep the whole tree, including tests,
  fixtures, config, and string or dynamic references.
- A shortcut marked with a `shortcut:` (or older `ponytail:`) comment that
  names its limit is a decision, not a finding, unless the expected load
  already crosses it.
- Propose the smallest fix that works. Prefer fixes that delete code. Never
  add layers, frameworks or config the problem does not need.
- No style taste, no "consider", no vague worries.

## 4. Output

Very simple English: short sentences, everyday words. Explain a technical
term the first time you use it. The reader may never have seen this code.

Start with `What this change does:` (change) or `What this repo does:` and
the load you assumed (repo), in two or three sentences.

Then the findings in three groups, most important first, skip empty groups:
- **Must fix:** bug, security, data loss, breaks at the expected load.
- **Should fix:** risky code without a test, real slowness, duplication, a
  function that mixes jobs, code that should not exist.
- **Nice to have:** small speed-ups, shorter forms.

Number findings across all groups, so the user can say "fix 2 and 5". Repo
scope: at most 20 findings; if you left smaller ones out, say how many.
Every finding has all four parts, each one or two short sentences:

2. **Orders land on the wrong day** (`billing/close_day.py:L40-52`)
   - **What this is:** At midnight this job closes the day and bills all orders of that day.
   - **Problem:** It takes "today" from the server clock, which runs in UTC. An order placed
     at 00:30 in Berlin is billed on the day before.
   - **Fix:** Compute the day once in the shop's time zone:
     `datetime.now(ZoneInfo("Europe/Berlin")).date()`. One line, nothing else changes.
   - **If we skip it:** Late orders show the wrong date, and accounting fixes them by hand.

End with:
- `Verdict: Ship.` / `Verdict: fix 1 and 3 first.` (change), or one line:
  healthy, or what to fix first (repo).
- `Lean: -<N> lines possible.` (repo: `-<N> lines, -<M> dependencies possible.`)
  when lean findings exist.
- `Not checked:` what mattered and you could not read or run.

Nothing found: the opening line, then `Looks good. Ship.` (change) or
`Healthy. Nothing to fix.` (repo), and one line on what you checked.

Report only; changes no code.
