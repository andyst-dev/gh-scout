// Package report renders a scout Report as Markdown (for humans) or JSON
// (for machines and for feeding into other tooling).
package report

import (
	"errors"

	"github.com/andyst-dev/gh-scout/internal/scout"
)

// Kind selects the report format.
type Kind string

const (
	// KindMarkdown is a human-readable summary.
	KindMarkdown Kind = "markdown"
	// KindJSON is the machine-readable representation.
	KindJSON Kind = "json"
)

// Writer renders a scout Report.
type Writer interface {
	// Write returns the rendered report ready to be streamed to its sink.
	Write(rep *scout.Report) ([]byte, error)
}

// New builds a Writer for the requested kind.
func New(kind Kind) (Writer, error) {
	switch kind {
	case KindMarkdown:
		return markdownWriter{}, nil
	case KindJSON:
		return jsonWriter{}, nil
	default:
		return nil, errors.New("unknown report format " + string(kind))
	}
}
