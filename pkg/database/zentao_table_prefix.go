package database

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	tablePrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	ZentaoTablePrefix  string
)

// SetZentaoTablePrefix configures the prefix used by ZenTao-owned tables.
// It must be called during database initialization before serving requests.
func SetZentaoTablePrefix(prefix string) error {
	normalized, err := normalizeTablePrefix(prefix)
	if err != nil {
		return err
	}
	ZentaoTablePrefix = normalized
	return nil
}

func normalizeTablePrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", fmt.Errorf("ZenTao table prefix is required; set database.zentaoTablePrefix in YAML")
	}
	if !tablePrefixPattern.MatchString(prefix) {
		return "", fmt.Errorf("invalid ZenTao table prefix %q: only letters, digits, and underscores are allowed", prefix)
	}
	return prefix, nil
}
