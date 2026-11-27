# adhd-simple

A text-only simplification of **[i-have-adhd](https://github.com/ayghri/i-have-adhd)
by Ayoub Ghriss**. It's for places where installing third-party plugins isn't
allowed. The ten rules and the overrides are i-have-adhd's own, rewritten in
shorter words, and all credit for them goes to the original. Go star it.

There is no plugin and no code: just a skill (Markdown files) you copy into place.

## What's kept, what's cut

| i-have-adhd | adhd-simple |
|---|---|
| the 10 rules, the "when to break the rules" overrides, the pre-send check | kept, reworded more tightly; plus an explicit `on`/`off` argument and one line asking to pass the rules on to subagents |
| "stop adhd mode" / "normal mode" | kept |
| always-on via a flag file and SessionStart hook | replaced by a `CLAUDE.md` import or an lplug rule (below) |
| evals, tests, translations | cut |
| Codex / Cursor / Gemini / Pi / OpenCode / … packaging | cut (Claude Code skills only) |

## Install

```sh
cp -R adhd-simple ~/.claude/skills/
```

With lplug, `skills.d/` is linked into `~/.claude/skills/` for you.

**Always on.** Add this line to `~/.claude/CLAUDE.md` and the rules load at
startup with no plugin and no hook:

```
@~/.claude/skills/adhd-simple/RULES.md
```

If you use lplug, `rules.d/adhd.ini` loads the rules and adds the on-track
hooks below. `lplug off @adhd` turns all of it off.

Without either, run `/adhd-simple` at the start of a session.

## Use

| Command | Does |
|---|---|
| `/adhd-simple [on\|off]` | turn it on (default) or pause it |

Saying "stop adhd mode" or "normal mode" also turns it off.

## Staying on track (added here; not in i-have-adhd)

You can go off course; the agent brings you back. It keeps one **main task**,
marks detours (`Detour from: …` / `Back to: …`), parks side ideas
(`Parked: …`), and before it wraps up lists what's `Still open:`.

The rules alone ask for this. With lplug, `rules.d/adhd.ini` adds hooks that make it reliable:

| Rule | When | Does |
|---|---|---|
| `adhd-detour` | your prompt starts with "oh / also / wait" or says "btw", "unrelated", "quick question"… | reminds the agent to mark the detour and come back |
| `adhd-nudge` | every 6th prompt | asks the agent to restate the main task |
| `adhd-return` | a reply has `Detour from:` but no `Back to:` | sends it back to add the way back |
| `adhd-finish` | a reply says done / finished / all set without `Still open:` | sends it back to list what's open |
| `adhd-compact` | before context compaction | keeps the main task and parked list in the summary |

Too naggy? `lplug off adhd-nudge`. Want only the reply style? `lplug off adhd-detour adhd-nudge adhd-return adhd-finish`.

## Works with ponytail

The two don't overlap: ponytail decides **what code** to write, adhd-simple
decides **how the reply reads**. Run both.

## License

MIT. The original copyright and permission notice are in [ADHD-LICENSE](ADHD-LICENSE),
as the license requires. This adaptation is distributed under the same terms.
