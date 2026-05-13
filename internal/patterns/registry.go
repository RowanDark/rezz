package patterns

import (
	"fmt"
	"regexp"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/RowanDark/rezz/internal/patterns/kits"
)

var kitFiles = map[string]string{
	"api-keys":    "api-keys.yaml",
	"credentials": "credentials.yaml",
	"endpoints":   "endpoints.yaml",
	"financial":   "financial.yaml",
	"javascript":  "javascript.yaml",
	"headers":     "headers.yaml",
	"cloud":       "cloud.yaml",
}

type Registry struct {
	patterns []Pattern
	log      *zap.Logger
}

func New(log *zap.Logger) *Registry {
	return &Registry{log: log}
}

func (r *Registry) Load(kitNames []string) error {
	if len(kitNames) == 1 && kitNames[0] == "all" {
		kitNames = make([]string, 0, len(kitFiles))
		for k := range kitFiles {
			kitNames = append(kitNames, k)
		}
	}
	for _, name := range kitNames {
		filename, ok := kitFiles[name]
		if !ok {
			return fmt.Errorf("unknown pattern kit %q: choices are api-keys, credentials, endpoints, financial, javascript, headers, cloud, all", name)
		}
		data, err := kits.FS.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("reading kit %q: %w", name, err)
		}
		var kf kitFile
		if err := yaml.Unmarshal(data, &kf); err != nil {
			return fmt.Errorf("parsing kit %q: %w", name, err)
		}
		loaded := 0
		for _, p := range kf.Patterns {
			compiled, err := regexp.Compile(p.Regex)
			if err != nil {
				r.log.Warn("pattern failed to compile",
					zap.String("kit", name),
					zap.String("pattern", p.Name),
					zap.Error(err),
				)
				continue
			}
			p.compiled = compiled
			r.patterns = append(r.patterns, p)
			loaded++
		}
		r.log.Info("kit loaded", zap.String("kit", name), zap.Int("patterns", loaded))
	}
	return nil
}

func (r *Registry) Count() int { return len(r.patterns) }
