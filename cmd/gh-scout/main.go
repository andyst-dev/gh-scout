// Command gh-scout finds contribution-worthy GitHub issues by filtering out
// issues an open pull request already targets and scoring how fixable the rest
// look.
//
// Usage:
//
//	gh-scout [flags] owner/repo [owner/repo ...]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
	"github.com/andyst-dev/gh-scout/internal/prs"
	"github.com/andyst-dev/gh-scout/internal/report"
	"github.com/andyst-dev/gh-scout/internal/scout"
)

var version = "dev"

// banner is printed to stderr on interactive runs and reused as the README
// header visual. Generated with figlet -f slant "gh scout". Stored per line
// because the glyph contains a backtick, so it cannot be a raw string literal.
var banner = strings.Join([]string{
	"          __                             __",
	"   ____ _/ /_     ______________  __  __/ /_",
	"  / __ `/ __ \\   / ___/ ___/ __ \\/ / / / __/",
	" / /_/ / / / /  (__  ) /__/ /_/ / /_/ / /_",
	" \\__, /_/ /_/  /____/\\___/\\____/\\__,_/\\__/",
	"/____/",
}, "\n")

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gh-scout:", err)
		os.Exit(1)
	}
}

// run is the whole CLI, isolated from main for testability.
func run(args []string, stdout *os.File) error {
	// The prs subcommand has its own flag set and pipeline.
	if len(args) > 0 && args[0] == "prs" {
		return runPRs(args[1:], stdout)
	}

	fs := flag.NewFlagSet("gh-scout", flag.ContinueOnError)
	var (
		sinceDays  = fs.Int("since", 14, "only issues created within this many days (0 disables)")
		labels     = fs.String("labels", "", "comma-separated labels to keep (e.g. bug,help wanted)")
		maxPerRepo = fs.Int("max", 50, "max issues examined per repository")
		minScore   = fs.Int("min-score", 0, "drop candidates scoring below this")
		format     = fs.String("format", "markdown", "output format: markdown or json")
		showVer    = fs.Bool("version", false, "print version and exit")
		rulesFile  = fs.String("rules-file", "", "JSON file overriding scoring weights (see `gh-scout rules-template`)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showVer {
		_, _ = fmt.Fprintln(stdout, "gh-scout", version)
		return nil
	}

	// gh-scout rules-template prints a copy of the default rules to edit.
	if len(fs.Args()) == 1 && fs.Arg(0) == "rules-template" {
		data, err := scout.DefaultRules().JSON()
		if err != nil {
			return err
		}
		_, _ = stdout.Write(data)
		_, _ = fmt.Fprintln(stdout)
		return nil
	}

	// Optional overrides for the scoring weights and the ready threshold.
	var rules scout.Rules
	if *rulesFile != "" {
		data, err := os.ReadFile(*rulesFile)
		if err != nil {
			return fmt.Errorf("read rules file: %w", err)
		}
		if rules, err = scout.RulesFromJSON(data); err != nil {
			return err
		}
	}

	// The report goes to stdout and stays machine-clean. Branding goes to
	// stderr, only when the terminal is interactive and the output is not JSON.
	if *format != "json" && isTerminal(os.Stderr) {
		_, _ = fmt.Fprint(os.Stderr, banner)
		_, _ = fmt.Fprintln(os.Stderr) // end the final art line
		_, _ = fmt.Fprintln(os.Stderr, "  gh-scout · contribution issues without duplicate work")
		_, _ = fmt.Fprintln(os.Stderr)
		_, _ = fmt.Fprintln(os.Stderr, "  score    0-100 = fixability | defect +25, repro +20, tests +15, good-first-issue +20")
		_, _ = fmt.Fprintln(os.Stderr, "  status   ready (score ≥ 35, no open PR) · addressed (an open PR links it) · unclear (< 35, skipped)")
		_, _ = fmt.Fprintln(os.Stderr, "  target   [score · easy/medium/hard] with a suggested PR line")
		_, _ = fmt.Fprintln(os.Stderr)
	}

	repos := fs.Args()
	if len(repos) == 0 {
		return errors.New("no repositories given (e.g. gh-scout owner/name [owner/name ...])")
	}
	for _, r := range repos {
		if !strings.Contains(r, "/") {
			return fmt.Errorf("repository %q must be owner/name", r)
		}
	}

	opts := scout.Options{
		Since:      time.Duration(*sinceDays) * 24 * time.Hour,
		Labels:     splitComma(*labels),
		MaxPerRepo: *maxPerRepo,
		MinScore:   *minScore,
		Rules:      rules,
	}

	runner := scout.NewRunner(github.New(""))
	rep, err := runner.Run(context.Background(), repos, opts)
	if err != nil {
		return err
	}

	writer, err := report.New(report.Kind(*format))
	if err != nil {
		return err
	}
	out, err := writer.Write(rep)
	if err != nil {
		return err
	}
	_, err = stdout.Write(out)
	return err
}

// runPRs scouts the user's own open pull requests and reports, for each,
// whether someone has responded since their last push and whether it is up to
// date with its base branch.
func runPRs(args []string, stdout *os.File) error {
	fs := flag.NewFlagSet("gh-scout prs", flag.ContinueOnError)
	var (
		user    = fs.String("user", "", "pull request author to scout (auto-detected from token if empty)")
		repos   = fs.String("repos", "", "comma-separated owner/name filter")
		days    = fs.Int("days", 30, "drop pull requests not updated within this many days")
		max     = fs.Int("max", 50, "total pull requests to examine")
		format  = fs.String("format", "markdown", "output format: markdown or json")
		token   = fs.String("token", "", "GitHub token (defaults to GITHUB_TOKEN)")
		showVer = fs.Bool("version", false, "print version and exit")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showVer {
		_, _ = fmt.Fprintln(stdout, "gh-scout", version)
		return nil
	}

	client := github.New(*token)
	runner := prs.NewRunner(client)
	rep, err := runner.Run(context.Background(), prs.Options{
		User:  *user,
		Repos: splitComma(*repos),
		Since: time.Duration(*days) * 24 * time.Hour,
		Max:   *max,
	})
	if err != nil {
		return err
	}

	// Branding to stderr, only when the terminal is interactive and inside a
	// pipe the output is not JSON. The report itself stays machine-clean.
	if *format != "json" && isTerminal(os.Stderr) {
		_, _ = fmt.Fprint(os.Stderr, banner)
		_, _ = fmt.Fprintln(os.Stderr)
		_, _ = fmt.Fprintln(os.Stderr, "  gh-scout prs · your pull requests and who has responded")
		_, _ = fmt.Fprintln(os.Stderr, "  response  @author when someone other than you commented after your last push")
		_, _ = fmt.Fprintln(os.Stderr, "  status    up to date · behind base · conflicts · blocked · unstable · evaluating")
		_, _ = fmt.Fprintln(os.Stderr)
	}

	out, err := prs.Render(rep, report.Kind(*format))
	if err != nil {
		return err
	}
	_, err = stdout.Write(out)
	return err
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isTerminal reports whether f is attached to an interactive terminal. It is
// used to keep branding out of piped output without pulling in a dependency.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
