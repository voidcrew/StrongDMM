package ship

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

func sourceCrew(data []byte, typePath, field string) ([]CrewJob, bool, error) {
	mask := dmSourceMask(data, true)
	for _, loc := range dmTypeLine.FindAllIndex(mask, -1) {
		if strings.TrimSpace(string(mask[loc[0]:loc[1]])) != typePath {
			continue
		}
		end := len(data)
		if next := dmTypeLine.FindIndex(mask[loc[1]:]); next != nil {
			end = loc[1] + next[0]
		}
		assignment := regexp.MustCompile(`(?m)^[\t ]+` + regexp.QuoteMeta(field) + `[\t ]*=[\t ]*`).FindIndex(mask[loc[1]:end])
		if assignment == nil {
			return nil, false, nil
		}
		a := loc[1] + assignment[1]
		b := a
		if bytes.HasPrefix(mask[a:], []byte("null")) {
			return nil, true, nil
		}
		depth := 0
		for ; b < end; b++ {
			if mask[b] == '(' {
				depth++
			}
			if mask[b] == ')' {
				depth--
				if depth == 0 {
					b++
					break
				}
			}
		}
		jobs, e := parseCrew(string(data[a:b]))
		return jobs, true, e
	}
	return nil, false, fmt.Errorf("crew definition missing")
}

func rewriteLiteralList(data []byte, typePath, field, value string) ([]byte, error) {
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
		return nil, fmt.Errorf("cannot uniquely locate %s in %s", field, typePath)
	}
	block := mask[start:end]
	if regexp.MustCompile(`(?m)^[\t ]*#`).Match(block) {
		return nil, fmt.Errorf("%s has conditional definitions; edit its crew in code", typePath)
	}
	assignments := regexp.MustCompile(`(?m)^[\t ]+`+regexp.QuoteMeta(field)+`[\t ]*=[\t ]*`).FindAllIndex(block, -1)
	if len(assignments) > 1 {
		return nil, fmt.Errorf("multiple %s assignments in %s", field, typePath)
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
		text := "\t" + field + " = " + value + newline
		if insert == start {
			text = newline + text
		}
		return append(append(append([]byte{}, data[:insert]...), text...), data[insert:]...), nil
	}
	a := start + assignments[0][1]
	b := a
	if bytes.HasPrefix(mask[a:], []byte("list(")) {
		depth := 0
		for ; b < end; b++ {
			if mask[b] == '(' {
				depth++
			}
			if mask[b] == ')' {
				depth--
				if depth == 0 {
					b++
					break
				}
			}
		}
		if depth != 0 {
			return nil, fmt.Errorf("unterminated %s", field)
		}
	} else if bytes.HasPrefix(mask[a:], []byte("null")) {
		b += 4
	} else {
		return nil, fmt.Errorf("%s in %s is computed; use a literal list to edit it here", field, typePath)
	}
	lineEnd := b
	for lineEnd < end && mask[lineEnd] != '\n' {
		lineEnd++
	}
	if strings.TrimSpace(string(mask[b:lineEnd])) != "" {
		return nil, fmt.Errorf("%s in %s is computed", field, typePath)
	}
	return append(append(append([]byte{}, data[:a]...), value...), data[b:]...), nil
}
