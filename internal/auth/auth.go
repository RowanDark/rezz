package auth

import "github.com/RowanDark/v0x/internal/config"

// Strategy defines the interface for authentication strategies.
type Strategy interface {
	Apply(cfg *config.Config) error
}
