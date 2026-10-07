# gh-scout

**Find contribution-worthy GitHub issues - without duplicating someone else's in-progress work.**

`gh-scout` scans a repository's open issues, filters out the ones an open pull
request already targets, scores how "fixable" the rest look, and prints a ranked,
motivated shortlist for an OSS contributor to pick from.

```
          __                             __
   ____ _/ /_     ______________  __  __/ /_
  / __ `/ __ \   / ___/ ___/ __ \/ / / / __/
 / /_/ / / / /  (__  ) /__/ /_/ / /_/ / /_
 \__, /_/ /_/  /____/\___/\____/\__,_/\__/
/____/
```

The ASCII header above is printed by `gh-scout` when you run it in a terminal.

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
$ gh-scout --since 30 acme/widgets acme/gadgets
...
## Ready targets · 4
- **[#482 · score 78 · easy]** App crashes on empty config (nil deref on load)
  - suggested PR: fix the null path: add a regression test, then the patch
  - possible duplicate: #910 "fix(auth): redact token in failure logs"
...
### Excluded - already addressed (2)
- acme/gadgets#151 Slow pagination from a missing index - open PR #168 already references it
```

```
$ gh-scout --since 30 acme/widgets acme/gadgets
```
The ASCII header above goes to *stderr* and only when the output isn't JSON, so
reports stay clean when piped into a file or another tool.

## Screenshots

The ASCII header, as printed on an interactive terminal:

![gh-scout header](assets/banner.png)

A full scouting run against two example repositories:

![gh-scout run](assets/session.png)

Each target carries a **`score` (0-100)** - how fixable the issue looks, from
its title, body and labels. The exact weighting is tabled under
[How it decides](#how-it-decides).

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
| `--rules-file` | | JSON file overriding scoring weights |
| `--version` | | print version and exit |

Examples:

```sh
# A weekly pick list for two repos, Markdown
gh-scout --since 30 --labels bug,help wanted acme/widgets acme/gadgets

# Machine-readable output for another tool
gh-scout --format json --min-score 60 acme/widgets
```

### Tuning the rules

Every scoring weight and the `ready` threshold are configurable without
recompiling. Generate an editable template that starts from the current
defaults:

```sh
gh-scout rules-template > rules.json
```

Edit it (e.g. raise `ready_threshold` to demand more signal, or reweight
`title_defect`), then point the tool at it:

```json
{
  "ready_threshold": 50,
  "weights": {
    "title_defect": 40,
    "body_reproduction": 30
  }
}
```

```sh
gh-scout --rules-file rules.json acme/widgets
```

The example above only touches `ready_threshold`, `title_defect` and
`body_reproduction` - every other weight keeps its default, so a one-line change
is enough to shift the recommendations.

Valid thresholds are 0-100. `--min-score` still applies on top of the rules'
ready threshold.

## How it decides

Each issue becomes a **candidate** with one of three statuses:

- **`ready`** - no open PR references it and it scored at least the threshold
  (≥ 35 in the default ruleset) to start work.
- **`addressed`** - an open pull request at least mentions the issue
  (`Fixes #9`, `Closes #4`, or any `#N` in its body). Deliberately broad: a PR
  that links an issue is signalling that issue is being worked on.
- **`unclear`** - too little to judge; skipped from the ready set.

The **fixability score (0-100)** is a sum of fixed point deltas - purely
declarative rules, nothing inferred. The exact weighting lives in
[`internal/scout/score.go`](internal/scout/score.go):

| Signal | Points |
|---|---|
| title names a defect (`crash` `panic` `leak` `null` `broken` `deadlock` …) | **+25** |
| body has a reproduction or code sample | **+20** |
| body describes the problem (≥ 60 chars) | **+10** |
| mentions tests / expected behaviour | **+15** |
| label `good first issue` | **+20** |
| label `help wanted` | **+10** |
| label `bug` | **+15** |
| title is vague (`bug` `issue` `problem` …) | **-10** |
| title is a question | **-20** |
| empty / very thin body (< 40 chars) | **-15** |
| `feature` / `enhancement` / `request` label | **-25** |

The total is clamped to **0-100**. A candidate becomes **`ready`** only at
**≥ 35** (`--min-score` raises the bar further); below that it is `unclear` and
skipped.

**Difficulty** (`easy` / `medium` / `hard`), decided in `advice.go`:

| Difficulty | They get it when |
|---|---|
| `easy` | label `good first issue` / `help wanted`, or a typo-flavoured title, or the body reproduces *and* mentions tests |
| `hard` | `feature`/`enhancement` label, an architectural title (`refactor` `redesign` `support`), or a long body with no reproduction |
| `medium` | everything else |

Each ready target also carries a one-line **suggested PR** hint picked from the
same signals (e.g. *"fix the crash path: add a regression test, then the
patch"*). It is a heuristic aid - still read the issue before starting.

**Worked example** - *"App crashes on startup with null pointer"*, body with a
`Steps to reproduce` block and `expected: no panic`, labelled `good first issue`:

| Signal | Points |
|---|---|
| title names a defect (`crash`) | +25 |
| body reproduces the problem | +20 |
| mentions tests / expected behaviour | +15 |
| label `good first issue` | +20 |
| **total** | **80** |

80 ≥ 35 → **`ready`**; no open PR references it → offered, difficulty `easy`,
suggested PR *"fix the crash path: add a regression test, then the patch"*
(the `crash` defect word in the title gives this the priority over the generic
repro suggestion).
An issue with none of those signals scores 0 and is **`unclear`** - it never
appears in the ready list.

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