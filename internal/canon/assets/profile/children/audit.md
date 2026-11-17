## Child repositories

A CODE parent lists each child repository in optional `govna/children.txt`:

```text
govna-children-v1
<parent-relative-path>
```

- Require the header, a final newline, and nonempty, unique, byte-sorted entries.
- Require each entry to be a slash-normalized path relative to the parent root, with `..` components only at its start.
- Reject absolute paths, backslashes, control characters, `.` components, and leading or trailing slashes.
- Require each entry to resolve to a real directory outside the parent worktree that does not contain the parent.
- Stop audit before emission when the registry is malformed or a child cannot be read.
- Stop audit and rm before emission when a DOC repository has the registry.
- Exclude the registry from rendered canon, canon baselines, ordinary audit drift, and target-only evidence.
- Compare each child's files with `govna render --child` for the parent's stack.
- Name each child file by its parent-relative path in the Markdown and JSON file lists.
- Force-sync a differing or missing child pointer or build tooling file.
- Give a differing child `.gitignore` the ordinary Director choices.
- Check only the canonical heading and two-column table shape of a child `CHANGELOG.md`.
- Classify a child `CHANGELOG.md` with a broken shape as `manual-fix`.
- Write the orphan note from `govna render --orphan` when the parent drops a child.

Accept a `govna/preserve.txt` entry that starts with `..` only when it names the `.gitignore` of a child declared in `govna/children.txt`, such as `../mud-engine/.gitignore`. Reject every other such entry, because a child's pointer and build tooling always update.
