package parser

import (
	"errors"
	"regexp"
	"strings"
)

type ParseResult struct {
	Version            string   `json:"version"`
	ConfigureArguments string   `json:"configureArguments"`
	Arguments          []string `json:"arguments"`
	Modules            []string `json:"modules"`
	BuiltBy            string   `json:"builtBy"`
	BuiltWith          string   `json:"builtWith"`
	Compiler           string   `json:"compiler"`
}

var (
	versionRe      = regexp.MustCompile(`nginx/([0-9]+\.[0-9.]+)`) // nginx/1.24.0
	plainVersionRe = regexp.MustCompile(`^[0-9]+\.[0-9.]+$`)
)

func ParseNginxV(output string) (*ParseResult, error) {
	result := &ParseResult{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "nginx version:") {
			if matches := versionRe.FindStringSubmatch(trimmed); len(matches) == 2 {
				result.Version = matches[1]
			}
			continue
		}
		if strings.Contains(trimmed, "configure arguments:") {
			idx := strings.Index(trimmed, "configure arguments:")
			result.ConfigureArguments = strings.TrimSpace(trimmed[idx+len("configure arguments:"):])
			continue
		}
		if strings.Contains(trimmed, "built by") {
			result.BuiltBy = trimmed
			continue
		}
		if strings.Contains(trimmed, "built with") {
			result.BuiltWith = trimmed
			continue
		}
		if strings.Contains(trimmed, "gcc") || strings.Contains(trimmed, "clang") {
			if result.Compiler == "" {
				result.Compiler = trimmed
			}
			continue
		}
	}

	if result.Version == "" {
		return nil, errors.New("无法解析 Nginx 版本，请确认输出中包含 nginx version")
	}
	if result.ConfigureArguments == "" {
		return nil, errors.New("无法解析 configure arguments，请确认输出包含 configure arguments")
	}

	args := splitShellArgs(result.ConfigureArguments)
	result.Arguments = args
	result.Modules = extractModules(args)

	return result, nil
}

func ValidVersion(version string) bool {
	return plainVersionRe.MatchString(strings.TrimSpace(version))
}

func extractModules(args []string) []string {
	var modules []string
	seen := map[string]struct{}{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--with-") || strings.HasPrefix(arg, "--add-module=") || strings.HasPrefix(arg, "--add-dynamic-module=") || strings.HasPrefix(arg, "--without-") {
			if _, ok := seen[arg]; ok {
				continue
			}
			seen[arg] = struct{}{}
			modules = append(modules, arg)
		}
	}
	return modules
}

func splitShellArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuotes := rune(0)
	escaped := false

	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}

	for _, r := range input {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case inQuotes != 0:
			if r == inQuotes {
				inQuotes = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			inQuotes = r
		case r == ' ' || r == '\n' || r == '\t':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return args
}
