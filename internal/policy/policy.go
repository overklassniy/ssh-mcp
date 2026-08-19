// Package policy implements command whitelist and blacklist validation
// using regular expressions, mirroring the security model of the original
// TypeScript ssh-mcp-server.
package policy

import (
	"fmt"
	"regexp"
)

// Policy validates commands against an optional whitelist and blacklist.
// If a whitelist is configured, a command must match at least one pattern.
// If a blacklist is configured, a command must not match any pattern.
// When both are present, the whitelist is checked first, then the blacklist.
type Policy struct {
	whitelist []*regexp.Regexp
	blacklist []*regexp.Regexp
}

// New creates a Policy from the given whitelist and blacklist pattern strings.
// Each pattern is compiled as a Go regular expression. An invalid pattern
// produces an error naming the pattern and the kind ("whitelist" or "blacklist").
func New(whitelist, blacklist []string) (*Policy, error) {
	p := &Policy{}
	if err := p.compile(whitelist, "whitelist"); err != nil {
		return nil, err
	}
	if err := p.compile(blacklist, "blacklist"); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Policy) compile(patterns []string, kind string) error {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid %s pattern %q: %w", kind, pattern, err)
		}
		if kind == "whitelist" {
			p.whitelist = append(p.whitelist, re)
		} else {
			p.blacklist = append(p.blacklist, re)
		}
	}
	return nil
}

// HasWhitelist reports whether a whitelist is configured.
func (p *Policy) HasWhitelist() bool {
	return len(p.whitelist) > 0
}

// HasBlacklist reports whether a blacklist is configured.
func (p *Policy) HasBlacklist() bool {
	return len(p.blacklist) > 0
}

// Validate checks whether a command is allowed by the policy.
// Returns allowed=true if the command passes both whitelist and blacklist checks.
// When not allowed, reason explains which check failed.
func (p *Policy) Validate(command string) (allowed bool, reason string) {
	if len(p.whitelist) > 0 {
		matched := false
		for _, re := range p.whitelist {
			if re.MatchString(command) {
				matched = true
				break
			}
		}
		if !matched {
			return false, "Command not in whitelist, execution forbidden"
		}
	}

	if len(p.blacklist) > 0 {
		for _, re := range p.blacklist {
			if re.MatchString(command) {
				return false, "Command matches blacklist, execution forbidden"
			}
		}
	}

	return true, ""
}
