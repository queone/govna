## Child Repositories

- Use this section for every repository listed in `govna/children.txt`.
- Treat a child as a separate Git repository that this parent imports at a pinned version and that has no roadmap of its own.
- Keep a repository with its own roadmap or a second developing consumer as a peer with full adoption.
- Expect a child to hold its pointer `AGENTS.md`, its stack's build tooling, `.gitignore`, `CHANGELOG.md`, and its own content.
- Make a new child pass its own `./build.sh` before the parent's next full build.
- Plan follow-up child work after `govna apply --child` as a second AC in the same release batch as the apply-child AC.
- Add a child to a Claude Code session with `--add-dir <path>` at startup or `/add-dir <path>` during the session.
- Add a child to a Codex CLI session with `--add-dir <path>` at startup.
- Drop a child with `govna rm --child` before running `govna apply` in the child to promote it to a peer.
