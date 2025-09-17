package regex

import (
	"fmt"
	"regexp"
	"strings"
)

// Processor handles regex pattern parsing, compilation, and automatic deduplication
type Processor struct {
	patterns []string
	compiled []*regexp.Regexp
	dirty    bool // Track if patterns need recompilation
}

// New creates a new regex processor
func New() *Processor {
	return &Processor{
		patterns: make([]string, 0),
		compiled: make([]*regexp.Regexp, 0),
		dirty:    true,
	}
}

// NewWithPatterns creates a new processor with the given patterns
func NewWithPatterns(patterns []string) *Processor {
	p := New()
	p.Add(patterns...)
	return p
}

// NewFromCommaSeparated creates a new processor with comma-separated patterns
func NewFromCommaSeparated(patterns string) *Processor {
	p := New()
	p.AddCommaSeparated(patterns)
	return p
}

// Add adds one or more patterns with automatic deduplication
func (p *Processor) Add(patterns ...string) {
	seen := make(map[string]bool)
	for _, existing := range p.patterns {
		seen[existing] = true
	}

	for _, pattern := range patterns {
		if pattern != "" && !seen[pattern] {
			p.patterns = append(p.patterns, pattern)
			seen[pattern] = true
			p.dirty = true
		}
	}
}

// AddCommaSeparated adds comma-separated patterns
func (p *Processor) AddCommaSeparated(patterns string) {
	parsed := parseCommaSeparated(patterns)
	p.Add(parsed...)
}

// Patterns returns the string patterns
func (p *Processor) Patterns() []string {
	return p.patterns
}

// Compiled returns the compiled regex patterns, compiling them if needed
func (p *Processor) Compiled() ([]*regexp.Regexp, error) {
	if p.dirty {
		if err := p.compile(); err != nil {
			return nil, err
		}
		p.dirty = false
	}
	return p.compiled, nil
}

// compile compiles all string patterns into regex patterns
func (p *Processor) compile() error {
	compiled := make([]*regexp.Regexp, len(p.patterns))
	for i, pattern := range p.patterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
		}
		compiled[i] = regex
	}
	p.compiled = compiled
	return nil
}

// parseCommaSeparated parses a comma-separated string of regex patterns into a slice
// Supports escaping commas with \, to allow commas within regex patterns
func parseCommaSeparated(patterns string) []string {
	if patterns == "" {
		return []string{}
	}

	// Replace escaped commas with a placeholder to preserve them
	placeholder := "\x00ESCAPED_COMMA\x00"
	patterns = strings.ReplaceAll(patterns, "\\,", placeholder)

	// Split by unescaped commas and trim whitespace
	parts := strings.Split(patterns, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			// Restore escaped commas
			restored := strings.ReplaceAll(trimmed, placeholder, ",")
			result = append(result, restored)
		}
	}
	return result
}
