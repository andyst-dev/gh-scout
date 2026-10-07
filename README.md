# gh-scout

**Find contribution-worthy GitHub issues - without duplicating someone else's in-progress work.**

`gh-scout` scans a repository's open issues, filters out the ones an open pull
request already targets, scores how "fixable" the rest look, and prints a ranked,
motivated shortlist for an OSS contributor to pick from.

[![go](https://img.shields.io/badge/go-1.27-00ADD8?logo=go)](https://go.dev)
[![CI](https://github.com/andyst-dev/gh-scout/workflows/ci/badge.svg)](https://github.com/andyst-dev/gh-scout/actions)
[![lint](https://img.shields.io/badge/golangci--lint-clean-2fbf4e?logo=go)](https://golangci-lint.run)
[![license](https://img.shields.io/badge/license-MIT-blue)](#license)

---

## Why

Picking a "good first issue" is easy. Picking a *good first issue that isn't
already being fixed by someone else* is the part that wastes contributors'
time. A pull request that `Fixes #123` makes issue #123 a trap: you start it,
then someone closes it as a duplicate.

That trap is exactly what `gh-scout` removes. It never posts anything - it
only tells you, for each issue, whether an open PR already covers it and how
much signal the issue gives to start work on it.

```
$ gh-scout --since 30 acme/widgets
...
## Ready targets · 4
- **[#2090 · score 75]** A bug finding's suggested fix breaks a Kody Rule…
  - possible duplicate: #2046 "fix(cli-review): thread BYOK slot into CLI review pipeline"
...
### Excluded - already addressed (3)
- acme/widgets#2060 Abandoned Azure PRs remain open … - open PR #2058 already references it
```

## Install

```sh
go install github.com/andyst-dev/gh-scout/cmd/gh-scout@latest
```

No external dependencies; the standard library is all it needs.

## Usage

```sh
gh-scout [flags] owner/repo [owner/repo ...]
```

A GitHub token is read from `GITHUB_TOKEN` to raise the API quota. Anonymous
use works but rate-limits quickly. Run `gh-scout --help` for the live flag
reference.

| Flag | Default | Meaning |
|---|---|---|
| `--since` | `14` | only issues created within this many days (`0` disables) |
| `--labels` | | comma-separated labels to keep, e.g. `bug,help wanted` |
| `--max` | `50` | max issues examined per repository |
| `--min-score` | `0` | drop targets scoring below this |
| `--format` | `markdown` | `markdown` or `json` |
| `--version` | | print version and exit |

Examples:

```sh
# A weekly pick list for two repos, Markdown
gh-scout --since 30 --labels bug,help wanted acme/widgets acme/gadgets

# Machine-readable output for another tool
gh-scout --format json --min-score 60 acme/widgets
```

## How it decides

Each issue becomes a **candidate** with one of three statuses:

- **`ready`** - no open PR references it and it scored at least the threshold
  (≥ 35 in the default ruleset) to start work.
- **`addressed`** - an open pull request at least mentions the issue
  (`Fixes #9`, `Closes #4`, or any `#N` in its body). Deliberately broad: a PR
  that links an issue is signalling that issue is being worked on.
- **`unclear`** - too little to judge; skipped from the ready set.

The **fixability score** (0-100) rewards concrete signals - a title describing
a defect, a reproduction/code sample in the body, mention of tests, and help
labels - and discounts vagueness, feature requests, and question titles. The
weighting lives in [`internal/scout/score.go`](internal/scout/score.go).

A *possible duplicate* is flagged, but not excluded, when an open PR has a
similar title without an explicit issue link.

### Limitations

- The score is **heuristic**, not a verdict. `ready` means "looks worth a
  read", never "merge this". Open the issue and judge before starting.
- The `addressed` check only sees **open** pull requests. An issue whose fix
  was merged (or abandoned) still shows as ready.
- Only issues are scanned from the repository's first four result pages, so
  the very oldest issues may be out of reach on huge repos.
- With no `GITHUB_TOKEN`, the anonymous quota (60 req/h) is spent in a couple
  of multi-repo runs.

## Project layout

```
cmd/gh-scout/     CLI entry point, flag wiring
internal/github/  minimal REST client (tokened, paginated)
internal/scout/   orchestration, anti-duplicate matching, scoring
internal/report/  Markdown + JSON renderers
```

`scout` depends only on an `IssueLister` interface, so the whole decision
layer is unit-tested against a fake (no HTTP mocking).

## Development

```sh
make test        # go test ./...
make lint        # golangci-lint run ./...
make fmt-check   # gofmt cleanliness gate
make build       # build bin/gh-scout
```

## License

[MIT](LICENSE)