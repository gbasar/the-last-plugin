---
name: query
description: Answer a question from the shared Obsidian wiki with [[wikilink]] citations, reading page summaries before page bodies. Read-only. Use when the user says "ask the wiki", "what do we know about", "check the wiki", "search the wiki", or /wiki:query.
argument-hint: "[@vault] <question>"
---

# Wiki query

Answer from the wiki, citing the pages used. **This skill never writes to the vault.**

**First read `../ingest/FORMAT.md`** (next to this skill's folder) for the vault
location and page format.

Question: $ARGUMENTS

## Steps: cheapest first, stop as soon as you can answer

1. **Find the vault** (FORMAT.md). If it has no `index.md`, say the wiki is empty
   and suggest `/wiki:ingest`.
2. **Index pass.** Read `index.md`: every page with its summary and tags. Pick
   the candidates.
3. **Frontmatter pass.** Grep only the frontmatter fields of the vault's `.md` files
   for the question's key terms and their synonyms, for example
   `^(title|summary|tags|aliases):.*<term>` plus the line after a `>-`. Skip
   `.obsidian/`, `_archives/`, `_raw/`.
   - If the summaries already answer it, answer and label it
     "(from page summaries only)".
4. **Section pass.** Grep page bodies for the terms with a few lines of context,
   and read just those sections.
5. **Full read**, last resort: read the 3 to 5 most relevant pages in full. Follow
   `[[wikilinks]]` one hop when the answer clearly continues on the linked page.
6. **Answer.**
   - Lead with the direct answer, then the supporting points.
   - Cite each point with the page it came from: `[[concepts/tool-calling]]`.
   - Keep `^[inferred]` and `^[ambiguous]` claims marked as such; never present
     them as fact.
   - Say which pass the answer came from (summaries, sections, or full pages).
   - If the wiki doesn't cover it, say so plainly and name the closest pages. Don't
     fill the gap from general knowledge unless the user asks, and then label it
     "not from the wiki".
   - If pages contradict each other, show both with their citations.
7. If the answer looks worth keeping as its own page, suggest
   `/wiki:ingest this conversation`, but don't write it yourself.
