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

// NewFromNewlineSeparated creates a new processor with newline-separated patterns
func NewFromNewlineSeparated(patterns string) *Processor {
	p := New()
	p.AddNewlineSeparated(patterns)
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

// AddNewlineSeparated adds newline-separated patterns
func (p *Processor) AddNewlineSeparated(patterns string) {
	parsed := parseNewlineSeparated(patterns)
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

// parseNewlineSeparated parses a newline-separated string of regex patterns into a slice
func parseNewlineSeparated(patterns string) []string {
	if patterns == "" {
		fmt.Printf("DEBUG: parseNewlineSeparated called with empty patterns\n")
		return []string{}
	}

	fmt.Printf("DEBUG: parseNewlineSeparated called with patterns: %q\n", patterns)

	// Split by newlines and trim whitespace
	parts := strings.Split(patterns, "\n")
	fmt.Printf("DEBUG: split into %d parts: %v\n", len(parts), parts)

	result := make([]string, 0, len(parts))
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			fmt.Printf("DEBUG: adding pattern[%d]: %q\n", i, trimmed)
			result = append(result, trimmed)
		} else {
			fmt.Printf("DEBUG: skipping empty pattern[%d]: %q\n", i, part)
		}
	}

	fmt.Printf("DEBUG: parseNewlineSeparated returning %d patterns: %v\n", len(result), result)
	return result
}
