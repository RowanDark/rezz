package output

import (
	"io"

	"github.com/RowanDark/v0x/internal/config"
)

// Formatter defines the interface for output formatters.
type Formatter interface {
	Write(w io.Writer, words []string, cfg *config.Config) error
}
