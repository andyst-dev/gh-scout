package prs

import (
	"errors"

	"github.com/andyst-dev/gh-scout/internal/report"
)

// Render renders a PR scout Report in the requested format. It reuses
// report.Kind so the CLI stays consistent across subcommands.
func Render(rep *Report, kind report.Kind) ([]byte, error) {
	switch kind {
	case report.KindMarkdown:
		return writeMarkdown(rep)
	case report.KindJSON:
		return writeJSON(rep)
	default:
		return nil, errors.New("unknown report format " + string(kind))
	}
}
