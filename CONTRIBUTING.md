# Contributing

When your pull request is ready, ask one of these teams for a review. One
approval is enough.

- @spacelift-io/foundations-backend
- @spacelift-io/enterprise-ready-backend
- @spacelift-io/ecosystem-backend

## Pull request titles

We squash-merge, therefore the title becomes the commit message on main, and
GitHub won't let you merge unless it's a Conventional Commit like
"fix(pollclient): back off after a failed poll".

The type picks the next version:

| Type | Release |
|---|---|
| feat | minor |
| fix, perf, refactor, revert, chore | patch |
| build, ci, docs, style, test | none |

Breaking changes get a ! before the colon, as in "feat(messages)!: drop the
unknown message handler", which is a minor bump while we're on v0. Put it in
the title. A ! on one of the commits inside the pull request is ignored.
