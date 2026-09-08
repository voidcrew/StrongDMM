package ship

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Mask comments (and optionally strings) without changing source offsets.
// This lets the small slot-list edit ignore examples in comments and strings.
func dmSourceMask(data []byte, maskStrings bool) []byte {
	out := append([]byte{}, data...)
	quote, block, line := byte(0), 0, false
	blank := func(i int) {
		if out[i] != '\n' && out[i] != '\r' {
			out[i] = ' '
		}
	}
	for i := 0; i < len(data); i++ {
		ch := data[i]
		next := byte(0)
		if i+1 < len(data) {
			next = data[i+1]
		}
		if line {
			blank(i)
			if ch == '\n' {
				line = false
			}
			continue
		}
		if block > 0 {
			blank(i)
			if ch == '/' && next == '*' {
				block++
				i++
				blank(i)
			} else if ch == '*' && next == '/' {
				block--
				i++
				blank(i)
			}
			continue
		}
		if quote != 0 {
			if maskStrings {
				blank(i)
			}
			if ch == '\\' && i+1 < len(data) {
				i++
				if maskStrings {
					blank(i)
				}
			} else if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '/' && (next == '/' || next == '*') {
			line = next == '/'
			if !line {
				block = 1
			}
			blank(i)
			i++
			blank(i)
		} else if ch == '"' || ch == '\'' {
			quote = ch
			if maskStrings {
				blank(i)
			}
		}
	}
	return out
}

var dmTypeLine = regexp.MustCompile(`(?m)^/[^\r\n]*`)
var roomSlotsAssignment = regexp.MustCompile(`(?m)^[\t ]+upgrade_slot_ids[\t ]*=[\t ]*`)

// Rewrite one literal slot list in its actual type block. Everything outside
// the expression, including jobs, prices, inheritance and procedures, is kept.
func rewriteRoomSlots(data []byte, typePath string, expected, slots []string) ([]byte, error) {
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
		return nil, fmt.Errorf("cannot uniquely locate the room list in %s", typePath)
	}
	block := mask[start:end]
	if regexp.MustCompile(`(?m)^[\t ]*#`).Match(block) {
		return nil, fmt.Errorf("%s has conditional definitions; use a literal upgrade_slot_ids list", typePath)
	}
	assignments := roomSlotsAssignment.FindAllIndex(block, -1)
	if len(assignments) > 1 {
		return nil, fmt.Errorf("multiple room lists in %s", typePath)
	}
	if len(assignments) == 0 {
		if reflect.DeepEqual(expected, slots) {
			return append([]byte{}, data...), nil
		}
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
		assignment := "\tupgrade_slot_ids = " + dmList(slots) + newline
		if insert == start {
			assignment = newline + assignment
		}
		return append(append(append([]byte{}, data[:insert]...), []byte(assignment)...), data[insert:]...), nil
	}
	valueStart := start + assignments[0][1]
	valueEnd := valueStart
	if bytes.HasPrefix(mask[valueStart:], []byte("list(")) {
		depth := 0
		for ; valueEnd < end; valueEnd++ {
			if mask[valueEnd] == '(' {
				depth++
			}
			if mask[valueEnd] == ')' {
				depth--
				if depth == 0 {
					valueEnd++
					break
				}
			}
		}
		if depth != 0 {
			return nil, fmt.Errorf("unterminated room list in %s", typePath)
		}
	} else if bytes.HasPrefix(mask[valueStart:], []byte("null")) {
		valueEnd += 4
	} else {
		return nil, fmt.Errorf("%s needs a literal upgrade_slot_ids list before adding rooms", typePath)
	}
	lineEnd := valueEnd
	for lineEnd < end && mask[lineEnd] != '\n' {
		lineEnd++
	}
	if strings.TrimSpace(string(mask[valueEnd:lineEnd])) != "" {
		return nil, fmt.Errorf("%s uses an expression for upgrade_slot_ids; use a literal list", typePath)
	}
	current, err := stringList(string(dmSourceMask(data[valueStart:valueEnd], false)))
	if err != nil || !reflect.DeepEqual(current, expected) {
		return nil, fmt.Errorf("%s room list differs from the loaded environment; reload the environment first", typePath)
	}
	if reflect.DeepEqual(current, slots) {
		return append([]byte{}, data...), nil
	}
	return append(append(append([]byte{}, data[:valueStart]...), []byte(dmList(slots))...), data[valueEnd:]...), nil
}
