# Agent and contributor operational policy

## Commit, push, tag, and release

- **AI agents may only create and edit code** (implementation, tests, fixtures, Makefile/CI YAML, documentation drafts, and RELEASE notes text when asked).
- **Humans commit, push, tag, and release.** Agents must not run `git commit`, `git push`, `git tag`, or create GitHub releases unless the human explicitly asks for that step.
- Agents stop at a reviewable working tree and summarise what is ready for the human to commit.

This policy is operational, not part of the CLI or e2e technical architecture.
