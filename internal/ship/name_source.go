package ship

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// Replace just the literal name, preserving comments and custom registrations.
func rewriteName(data []byte, typePath, expected, name string) ([]byte, error) {
	mask := dmSourceMask(data, true)
	start, end, matches := -1, len(data), 0
	for _, loc := range dmTypeLine.FindAllIndex(mask, -1) {
		if strings.TrimSpace(string(mask[loc[0]:loc[1]])) == typePath {
			start = loc[1]
			matches++
		} else if start >= 0 && end == len(data) {
			end = loc[0]
		}
	}
	if matches != 1 || end < start {
		return nil, fmt.Errorf("cannot uniquely locate the name in %s", typePath)
	}
	block := mask[start:end]
	if regexp.MustCompile(`(?m)^[\t ]*#`).Match(block) {
		return nil, fmt.Errorf("%s has conditional definitions; edit its name in code", typePath)
	}
	assignments := regexp.MustCompile(`(?m)^[\t ]+name[\t ]*=`).FindAllIndex(block, -1)
	if len(assignments) > 1 {
		return nil, fmt.Errorf("multiple name assignments in %s", typePath)
	}
	if len(assignments) == 0 {
		newline := "\n"
		if bytes.Contains(data, []byte("\r\n")) {
			newline = "\r\n"
		}
		insert := start
		if insert < len(data) && data[insert] == '\r' {
			insert++
		}
		if insert < len(data) && data[insert] == '\n' {
			insert++
		}
		assignment := "\tname = " + dmQuote(name) + newline
		if insert == start {
			assignment = newline + assignment
		}
		return append(append(append([]byte{}, data[:insert]...), assignment...), data[insert:]...), nil
	}
	a := start + assignments[0][1]
	for a < end && (data[a] == ' ' || data[a] == '\t') {
		a++
	}
	if a >= end || data[a] != '"' {
		return nil, fmt.Errorf("%s needs a literal name to rename it here", typePath)
	}
	b := a + 1
	for b < end && data[b] != '"' {
		if data[b] == '\\' {
			b++
		}
		b++
	}
	if b >= end {
		return nil, fmt.Errorf("unterminated name in %s", typePath)
	}
	b++
	lineEnd := b
	for lineEnd < end && mask[lineEnd] != '\n' {
		lineEnd++
	}
	if strings.TrimSpace(string(mask[b:lineEnd])) != "" {
		return nil, fmt.Errorf("%s uses a computed name; edit it in code", typePath)
	}
	current, err := dmUnquote(string(data[a:b]))
	if err != nil || current != expected {
		return nil, fmt.Errorf("%s name differs from the loaded environment; reload the environment first", typePath)
	}
	return append(append(append([]byte{}, data[:a]...), dmQuote(name)...), data[b:]...), nil
}
