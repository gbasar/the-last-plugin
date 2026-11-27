# wiki

A shared Obsidian wiki that Claude keeps up to date. Two skills, no scripts, nothing to install:

| command | what it does |
|---|---|
| `/wiki:ingest <file, folder, URL or "this conversation">` | distills it into linked pages, merging into existing ones, and updates `index.md` and `log.md` |
| `/wiki:query <question>` | answers from the wiki with `[[wikilink]]` citations, reading summaries before pages; never writes |

## Install (each teammate)

```text
/plugin marketplace add <this repo's git URL or local path>
/plugin install wiki@the-last-plugin
```

## Point it at the vault

Either set `WIKI_VAULT=/path/to/vault`, or let the first `/wiki:...` ask you; it
saves the path in `~/.config/wiki/vault`. `@path` in a request uses another vault
once: `/wiki:query @~/team-wiki how do we deploy?`

## Format

Pages live in category folders (`concepts/ entities/ skills/ references/
synthesis/ journal/ projects/<name>/`) with frontmatter `title category tags
sources summary created updated`, `[[wikilinks]]` between pages, and
`^[inferred]` / `^[ambiguous]` on claims a source doesn't state outright. The full
rules are in [skills/ingest/FORMAT.md](skills/ingest/FORMAT.md).

The format follows [obsidian-wiki](https://github.com/Ar9av/obsidian-wiki) (MIT),
so a vault works with both. It's OKF-shaped (category folders, `index.md`,
`log.md`); an OKF tool that understands wikilinks can read it as is.

Not included yet: an ingest manifest, lint, dedup, tag taxonomy, OKF export.
