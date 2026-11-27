# ponytail-simple

A text-only simplification of **[ponytail](https://github.com/DietrichGebert/ponytail)
by DietrichGebert**. It's for places where installing third-party plugins isn't
allowed. The rules, the levels and the review method are ponytail's own, and
all credit for them goes to the original. Go star it.

There is no plugin and no code: just skills (Markdown files) you copy into place.

## What's kept, what's cut

| ponytail | ponytail-simple |
|---|---|
| core rules, levels lite / full / ultra | kept word for word, apart from: the credit line, the switch command (`/ponytail-simple`), an explicit `off` level, and one line asking to pass the level on to subagents (ponytail does that with a hook) |
| `ponytail-review` + `ponytail-audit` | merged into **`ponytail-simple-review`**: one checklist, two scopes (change or repo) |
| `ponytail-debt` | kept as **`ponytail-simple-debt`** |
| level switching and state via hooks | the skill sets the level; it lasts for the session |
| automatic codebase map at start | cut (the rules already say to look for existing code first) |
| status band, task board, webhooks | cut |
| `ponytail-gain`, `ponytail-help` | cut (this README is the help) |
| Codex / Cursor / Copilot / Gemini packaging | cut (Claude Code skills only) |

## Install

```sh
cp -R ponytail-simple ponytail-simple-review ponytail-simple-debt ~/.claude/skills/
```

**Always on (recommended).** Ponytail is meant to be active every session.
Add this line to `~/.claude/CLAUDE.md`, and the rules load at startup with
no plugin and no hook:

```
@~/.claude/skills/ponytail-simple/RULES.md
```

If you use lplug, a rule does the same and can be toggled with `lplug off ponytail`:

```ini
[ponytail]
event = session_start
file  = ~/.claude/skills/ponytail-simple/RULES.md
```

Without either, run `/ponytail-simple` at the start of a session.

## Use

| Command | Does |
|---|---|
| `/ponytail-simple [lite\|full\|ultra\|off]` | set the level (default full) |
| `/ponytail-simple-review [change\|repo\|path\|PR]` | review a change, or audit a repo |
| `/ponytail-simple-debt [word]` | list every `shortcut:` comment |

Saying "stop ponytail" or "normal mode" also turns it off.

## License

MIT. The original copyright and permission notice are in [LICENSE](LICENSE),
as the license requires. This adaptation is distributed under the same terms.
