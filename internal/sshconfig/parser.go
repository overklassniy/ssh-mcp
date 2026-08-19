// Package sshconfig parses OpenSSH-style ~/.ssh/config files and resolves
// host aliases to their connection parameters (HostName, User, Port,
// IdentityFile). It supports Include directives, first-match-wins semantics,
// wildcard host patterns, and tilde expansion.
package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Entry holds the resolved SSH configuration for a single host alias.
// Fields that are not found in the config file remain zero-valued.
type Entry struct {
	HostName    string
	User        string
	Port        int
	IdentityFile string
}

// hostBlock represents a single "Host pattern1 pattern2" block.
type hostBlock struct {
	patterns []string
	config   map[string]string
}

// Lookup finds the SSH configuration for the given host alias.
// configPath defaults to ~/.ssh/config when empty.
// Returns nil if the alias does not match any block or the default
// config file does not exist. An explicit configPath that does not
// exist produces an error.
func Lookup(hostAlias, configPath string) (*Entry, error) {
	explicit := configPath != ""
	if configPath == "" {
		configPath = filepath.Join(homeDir(), ".ssh", "config")
	}

	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			if explicit {
				return nil, fmt.Errorf("ssh config file not found: %s", configPath)
			}
			return nil, nil
		}
		return nil, fmt.Errorf("ssh config file not accessible: %w", err)
	}

	blocks, err := parseFile(configPath, make(map[string]bool))
	if err != nil {
		return nil, err
	}
	return matchHost(hostAlias, blocks), nil
}

// parseFile reads and parses an SSH config file, following Include directives.
// visited prevents infinite Include cycles by tracking real paths.
func parseFile(path string, visited map[string]bool) ([]hostBlock, error) {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		realPath = path
	}
	if visited[realPath] {
		return nil, nil
	}
	visited[realPath] = true

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var blocks []hostBlock
	var current *hostBlock

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)

		// Include directive
		if strings.HasPrefix(lower, "include ") {
			if current != nil {
				blocks = append(blocks, *current)
				current = nil
			}
			pattern := strings.TrimSpace(line[len("include "):])
			paths := expandInclude(pattern, filepath.Dir(path))
			for _, incPath := range paths {
				incBlocks, err := parseFile(incPath, visited)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, incBlocks...)
			}
			continue
		}

		// Host directive
		if strings.HasPrefix(lower, "host ") {
			if current != nil {
				blocks = append(blocks, *current)
			}
			patterns := strings.Fields(line[len("host "):])
			current = &hostBlock{patterns: patterns, config: make(map[string]string)}
			continue
		}

		// Key-value directive
		if current == nil {
			current = &hostBlock{patterns: []string{"*"}, config: make(map[string]string)}
		}
		spaceIdx := strings.IndexAny(line, " \t")
		if spaceIdx == -1 {
			continue
		}
		key := strings.ToLower(line[:spaceIdx])
		val := strings.TrimSpace(line[spaceIdx+1:])
		// First-match-wins: only keep the first value for each key.
		if _, exists := current.config[key]; !exists {
			current.config[key] = val
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}

	if current != nil {
		blocks = append(blocks, *current)
	}
	return blocks, nil
}

// matchHost applies first-match-wins semantics across all blocks that match
// the alias, collecting the first value seen for each field.
func matchHost(hostAlias string, blocks []hostBlock) *Entry {
	result := &Entry{}
	found := false

	for _, block := range blocks {
		if !blockMatches(hostAlias, block.patterns) {
			continue
		}
		found = true
		if result.HostName == "" {
			if v, ok := block.config["hostname"]; ok {
				result.HostName = v
			}
		}
		if result.User == "" {
			if v, ok := block.config["user"]; ok {
				result.User = v
			}
		}
		if result.Port == 0 {
			if v, ok := block.config["port"]; ok {
				var p int
				fmt.Sscanf(v, "%d", &p)
				result.Port = p
			}
		}
		if result.IdentityFile == "" {
			if v, ok := block.config["identityfile"]; ok {
				result.IdentityFile = expandTilde(v)
			}
		}
	}

	if !found {
		return nil
	}
	return result
}

// blockMatches checks whether the alias matches any positive pattern and is
// not excluded by a negated pattern. A negated match causes an immediate
// false return.
func blockMatches(hostAlias string, patterns []string) bool {
	positiveMatch := false
	for _, pattern := range patterns {
		negated := strings.HasPrefix(pattern, "!")
		body := pattern
		if negated {
			body = pattern[1:]
		}
		if body == "" {
			continue
		}
		if patternMatches(hostAlias, body) {
			if negated {
				return false
			}
			positiveMatch = true
		}
	}
	return positiveMatch
}

// patternMatches converts an SSH host pattern (with * and ? wildcards) into
// a regex and tests the alias against it.
func patternMatches(hostAlias, pattern string) bool {
	if pattern == "*" {
		return true
	}
	regexSrc := regexp.QuoteMeta(pattern)
	regexSrc = strings.ReplaceAll(regexSrc, `\*`, `.*`)
	regexSrc = strings.ReplaceAll(regexSrc, `\?`, `.`)
	re, err := regexp.Compile("^" + regexSrc + "$")
	if err != nil {
		return false
	}
	return re.MatchString(hostAlias)
}

// expandInclude resolves an Include pattern relative to baseDir, expanding
// ~ and glob wildcards.
func expandInclude(pattern, baseDir string) []string {
	if strings.HasPrefix(pattern, "~/") {
		pattern = filepath.Join(homeDir(), pattern[2:])
	} else if pattern == "~" {
		pattern = homeDir()
	} else if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(baseDir, pattern)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}
	return matches
}

// expandTilde replaces a leading ~ with the home directory.
func expandTilde(p string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir(), p[2:])
	}
	if p == "~" {
		return homeDir()
	}
	return p
}

// stripComment removes everything from the first unescaped # to end of line.
func stripComment(line string) string {
	for i, c := range line {
		if c == '#' {
			// A # at the start or preceded by whitespace is a comment.
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return line[:i]
			}
		}
	}
	return line
}

// homeDir returns the user's home directory, falling back to "" on error.
func homeDir() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return dir
}
