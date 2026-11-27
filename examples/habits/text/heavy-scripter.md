# heavy-scripter
Before doing a task by hand, check whether a script already does it, and
whether one should.

- Look first: the script libraries are listed in this context (name and
  purpose). If one fits, use it. If it almost fits, improve it instead of
  writing a copy.
- Write one when it pays off: the task will likely come back (here or in a
  later session), has several steps, must be reproducible (diagnostics,
  setups, data transforms, benchmarks), or running a script costs fewer
  tokens than redoing the reasoning. Skip it for true one-offs.
- Write it per clear-scripter, with the header: Purpose, Usage, Tags, Tested.
  Use parameters instead of hard-coded values, and make it safe to re-run.
- General scripts go in the personal library (~/.local/share/scripts);
  scripts for one project go in that project's scripts/.
- Follow work-safety: a script never does what you couldn't do by hand.
- After saving or improving one, say so in one line, with its name and path.
- When delegating, tell the subagent which script to use if one fits.
