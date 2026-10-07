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
	fs := flag.NewFlagSet("gh-scout", flag.ContinueOnError)
	var (
		sinceDays  = fs.Int("since", 14, "only issues created within this many days (0 disables)")
		labels     = fs.String("labels", "", "comma-separated labels to keep (e.g. bug,help wanted)")
		maxPerRepo = fs.Int("max", 50, "max issues examined per repository")
		minScore   = fs.Int("min-score", 0, "drop candidates scoring below this")
		format     = fs.String("format", "markdown", "output format: markdown or json")
		showVer    = fs.Bool("version", false, "print version and exit")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showVer {
		_, _ = fmt.Fprintln(stdout, "gh-scout", version)
		return nil
	}

	// The report goes to stdout and stays machine-clean. Branding goes to
	// stderr, only when the terminal is interactive and the output is not JSON.
	if *format != "json" && isTerminal(os.Stderr) {
		_, _ = fmt.Fprint(os.Stderr, banner)
		_, _ = fmt.Fprintln(os.Stderr) // end the final art line
		_, _ = fmt.Fprintln(os.Stderr, "  gh-scout · contribution issues without duplicate work")
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
