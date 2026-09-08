package ship

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

func costListRange(data []byte, typePath, field string) (line, a, b int, found, explicit bool, err error) {
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
	if matches == 0 {
		return
	}
	found = true
	if matches != 1 || end < start {
		err = fmt.Errorf("cannot uniquely locate %s in %s", field, typePath)
		return
	}
	block := mask[start:end]
	if regexp.MustCompile(`(?m)^[\t ]*#`).Match(block) {
		err = fmt.Errorf("%s has conditional definitions; edit its costs in source", typePath)
		return
	}
	assignments := regexp.MustCompile(`(?m)^[\t ]+`+regexp.QuoteMeta(field)+`[\t ]*=[\t ]*`).FindAllIndex(block, -1)
	if len(assignments) == 0 {
		return
	}
	if len(assignments) != 1 {
		err = fmt.Errorf("multiple %s assignments in %s", field, typePath)
		return
	}
	explicit = true
	line, a = start+assignments[0][0], start+assignments[0][1]
	b = a
	if bytes.HasPrefix(mask[a:], []byte("null")) {
		b += 4
	} else if bytes.HasPrefix(mask[a:], []byte("list(")) {
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
			err = fmt.Errorf("unterminated %s in %s", field, typePath)
			return
		}
	} else {
		err = fmt.Errorf("%s in %s is computed; use a literal list to edit it here", field, typePath)
		return
	}
	lineEnd := b
	for lineEnd < end && mask[lineEnd] != '\n' {
		lineEnd++
	}
	if strings.TrimSpace(string(mask[b:lineEnd])) != "" {
		err = fmt.Errorf("%s in %s is computed", field, typePath)
	}
	return
}

func sourceCostList(data []byte, typePath, field string) (string, bool, bool, error) {
	_, a, b, found, explicit, err := costListRange(data, typePath, field)
	if err != nil || !explicit {
		return "", found, explicit, err
	}
	return string(data[a:b]), found, explicit, nil
}

func removeCostAssignment(data []byte, typePath, field string) ([]byte, error) {
	line, _, b, found, explicit, err := costListRange(data, typePath, field)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("cost definition missing for %s", typePath)
	}
	if !explicit {
		return data, nil
	}
	// Keep a trailing comment; only the inserted assignment is removed by undo.
	end := b
	for end < len(data) && (data[end] == ' ' || data[end] == '\t' || data[end] == '\r') {
		end++
	}
	if end < len(data) && data[end] == '\n' {
		end++
	} else {
		end = b
	}
	return append(append([]byte{}, data[:line]...), data[end:]...), nil
}
