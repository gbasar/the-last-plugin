---
name: ingest
description: Distill sources into the shared Obsidian wiki as linked pages with frontmatter categories. Use when the user says "ingest", "add to the wiki", "save this to the wiki", "capture this", or /wiki:ingest with a file, folder, URL, or "this conversation".
argument-hint: "[@vault] <file | folder | URL | this conversation>"
---

# Wiki ingest

Turn what the user points at into wiki pages: merge into pages that exist, create
the ones that don't, link them, and keep `index.md` and `log.md` current.

**First read `FORMAT.md` in this skill's folder.** It defines the vault location,
folders, page template, index and log. Follow it exactly.

Input: $ARGUMENTS (empty means "this conversation").

## Steps

1. **Find the vault** (FORMAT.md, "Finding the vault"). Read `index.md`; if the
   vault is new, create `index.md` and `log.md` with just their headings.
2. **Read the source.**
   - File: read it. Folder: list it, skip binaries, audio, video and anything over
     about 2 MB, and say what you skipped. More than 10 files: show the list and
     ingest in batches of 10, reporting after each.
   - URL: fetch it. Conversation: use what was worked out in this session, not the
     chatter: decisions, fixes, how-tos, gotchas.
   - Already ingested? Search `log.md` and pages' `sources:` for the source key. If
     it's there and the source hasn't changed, say so and stop unless the user
     asked to redo it.
3. **Extract:** the concepts, entities, procedures and claims worth keeping, and
   the open questions. Note for each claim whether the source states it, you
   inferred it, or it's ambiguous.
4. **Plan, then write.** Decide which pages to update and which to create, at most
   15 per source (aim for under 10; say what you deferred). For each one:
   - Check `index.md` and the category folders for an existing page on the same
     topic, including under another name. Prefer updating it to creating a
     near-duplicate.
   - Pick the category folder, and `projects/<name>/` if it only applies to one project.
5. **Write pages** with the FORMAT.md template.
   - New page: all required frontmatter, links to 2 or more related existing
     pages (when the vault has them), the claim markers, a `## Sources` section.
   - Existing page: read it first, merge the new information into the right
     sections (don't just append), note contradictions with `^[ambiguous]`, add the
     source to `sources:` and `## Sources`, update `updated`, and rewrite `summary`
     if the page's meaning changed. Never change `created`.
6. **Link both ways.** For each new link A → B, add B → A under B's `## Related`
   when it helps a reader.
7. **Rebuild `index.md`** from the frontmatter of every page on disk, and append
   one line to `log.md`.
8. **Report** in a few lines: pages created and updated (as wikilinks), anything
   skipped or deferred, and contradictions found.

Never delete a page. Never edit files outside the vault, other than the vault
config file from FORMAT.md.
