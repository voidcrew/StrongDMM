// Package dminclude edits project registrations without sorting existing code.
package dminclude

import (
	"path"
	"regexp"
	"strings"
)

var includeLine = regexp.MustCompile(`^[\t ]*#include[\t ]+"([^"\r\n]+)"`)
var directive = regexp.MustCompile(`^[\t ]*#[\t ]*(if|ifdef|ifndef|endif|else|elif)\b`)
var endIncludes = regexp.MustCompile(`(?m)^// END_INCLUDE[^\r\n]*`)

type entry struct {
	start, end, nameStart, nameEnd, depth int
	name                                  string
}

func canonical(name string) string {
	return strings.ToLower(path.Clean(strings.ReplaceAll(name, "\\", "/")))
}

// Add uses BYOND's backslash convention and groups a generated source with its
// directory. Only this file's registration is moved; other include order, line
// endings, comments and conditional scopes are preserved.
func Add(data []byte, relative string) []byte {
	name := path.Clean(strings.ReplaceAll(relative, "\\", "/"))
	key := canonical(name)
	if name == "." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
		return data
	}
	text := string(data)
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	entries := parse(text)
	line := "#include \"" + strings.ReplaceAll(name, "/", "\\") + "\"" + newline
	// An existing conditional registration must stay in its original scope.
	for _, e := range entries {
		if canonical(e.name) == key && e.depth != 0 {
			return []byte(text[:e.nameStart] + strings.ReplaceAll(e.name, "/", "\\") + text[e.nameEnd:])
		}
	}
	var matches []entry
	for _, e := range entries {
		if canonical(e.name) == key {
			if len(matches) == 0 {
				line = text[e.start:e.nameStart] + strings.ReplaceAll(e.name, "/", "\\") + text[e.nameEnd:e.end]
				if !strings.HasSuffix(line, "\n") {
					line += newline
				}
			}
			matches = append(matches, e)
		}
	}
	for i := len(matches) - 1; i >= 0; i-- {
		text = text[:matches[i].start] + text[matches[i].end:]
	}
	entries = parse(text)
	end := len(text)
	// The marker is itself a line comment, so only block comments are masked.
	if marker := endIncludes.FindStringIndex(string(mask([]byte(text), false))); marker != nil {
		end = marker[0]
	}
	// Prefer the largest existing contiguous group in the nearest directory.
	// Older workshop versions left stray includes at the bottom of the DME.
	bestDepth := -1
	var best, run []entry
	flush := func() {
		if len(run) > len(best) {
			best = append([]entry(nil), run...)
		}
		run = nil
	}
	for _, e := range entries {
		if e.depth != 0 || e.start >= end {
			flush()
			continue
		}
		depth := sharedDirectory(key, canonical(e.name))
		if depth > bestDepth {
			bestDepth, best, run = depth, nil, nil
		}
		if depth != bestDepth {
			flush()
			continue
		}
		if len(run) > 0 && strings.TrimSpace(string(mask([]byte(text[run[len(run)-1].end:e.start]), true))) != "" {
			flush()
		}
		run = append(run, e)
	}
	flush()
	at := end
	if len(best) > 0 {
		at = best[len(best)-1].end
		for _, e := range best {
			if before(key, canonical(e.name)) {
				at = e.start
				break
			}
		}
	}
	if at > 0 && text[at-1] != '\n' {
		line = newline + line
	}
	return []byte(text[:at] + line + text[at:])
}

func sharedDirectory(a, b string) int {
	x, y := strings.Split(path.Dir(a), "/"), strings.Split(path.Dir(b), "/")
	n := 0
	for n < len(x) && n < len(y) && x[n] == y[n] {
		n++
	}
	return n
}

// Dream Maker lists a directory's own files before its subdirectories.
func before(a, b string) bool {
	x, y := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(x) && i < len(y); i++ {
		if (i == len(x)-1) != (i == len(y)-1) {
			return i == len(x)-1
		}
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return false
}

func parse(text string) []entry {
	masked := string(mask([]byte(text), true))
	var entries []entry
	depth, offset := 0, 0
	for _, line := range strings.SplitAfter(masked, "\n") {
		if m := directive.FindStringSubmatch(line); m != nil {
			switch m[1] {
			case "if", "ifdef", "ifndef":
				depth++
			case "endif":
				depth = max(0, depth-1)
			}
		}
		if m := includeLine.FindStringSubmatchIndex(line); m != nil {
			entries = append(entries, entry{offset, offset + len(line), offset + m[2], offset + m[3], depth, text[offset+m[2] : offset+m[3]]})
		}
		offset += len(line)
	}
	return entries
}

// Blank comments while retaining offsets and quoted include paths. Line
// comments are kept when lines is false, which finds the END_INCLUDE marker.
func mask(data []byte, lines bool) []byte {
	out := append([]byte(nil), data...)
	block := 0
	quoted, line := false, false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if c == '\n' {
			// DM strings and line comments end with the line.
			line, quoted = false, false
			continue
		}
		if !line && block == 0 && c == '"' {
			quoted = !quoted
		}
		if quoted {
			if c == '\\' && i+1 < len(data) {
				i++
			}
			continue
		}
		if !line && i+1 < len(data) {
			pair := string(data[i : i+2])
			if pair == "/*" {
				block++
				out[i], out[i+1] = ' ', ' '
				i++
				continue
			}
			if block > 0 && pair == "*/" {
				block--
				out[i], out[i+1] = ' ', ' '
				i++
				continue
			}
			if block == 0 && lines && pair == "//" {
				line = true
			}
		}
		if (line || block > 0) && c != '\r' {
			out[i] = ' '
		}
	}
	return out
}
