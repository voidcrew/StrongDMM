package ship

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var removalPath = regexp.MustCompile(`/[A-Za-z_][A-Za-z_0-9]*(?:/[A-Za-z_0-9]+)*`)
var removalInclude = regexp.MustCompile(`(?m)^[\t ]*#include[\t ]+"([^"\r\n]+)"[^\r\n]*(?:\r?\n|$)`)

func ownedModuleMaps(data []byte, roots []string, directory string) ([]string, error) {
	if !bytes.Contains(data, []byte("map_file")) || !containsRemovalRoot(data, roots) {
		return nil, nil
	}
	mask := dmSourceMask(data, false)
	lines := dmTypeLine.FindAllIndex(dmSourceMask(data, true), -1)
	assignment := regexp.MustCompile(`(?m)^[\t ]+(map_file|for_theme)[\t ]*=[\t ]*([^\r\n]+)`)
	var files []string
	for i, line := range lines {
		path := strings.TrimSpace(string(data[line[0]:line[1]]))
		if !strings.HasPrefix(path, "/datum/ship_upgrade_module/") || !withinTypes(path, roots) {
			continue
		}
		end := len(mask)
		if i+1 < len(lines) {
			end = lines[i+1][0]
		}
		var file string
		var themes []string
		for _, match := range assignment.FindAllSubmatch(mask[line[1]:end], -1) {
			value := strings.TrimSpace(string(match[2]))
			var err error
			if string(match[1]) == "map_file" {
				file, err = dmUnquote(value)
			} else {
				themes, err = stringList(value)
			}
			if err != nil {
				return nil, fmt.Errorf("cannot read the saved maps for %s: %w", path, err)
			}
		}
		if file == "" {
			continue
		}
		files = append(files, filepath.Join(directory, file))
		for _, theme := range themes {
			files = append(files, filepath.Join(directory, strings.TrimSuffix(file, ".dmm")+"_"+theme+".dmm"))
		}
	}
	return files, nil
}

func ownedRegistrationTypes(data []byte, roots []string) []string {
	if !bytes.Contains(data, []byte("for_ship")) || !containsRemovalRoot(data, roots) {
		return nil
	}
	mask := dmSourceMask(data, true)
	lines := dmTypeLine.FindAllIndex(mask, -1)
	assignment := regexp.MustCompile(`(?m)^[\t ]+for_ship[\t ]*=[\t ]*(/[^\s;]+)`)
	var result []string
	for i, line := range lines {
		path := strings.TrimSpace(string(mask[line[0]:line[1]]))
		if strings.ContainsAny(path, "( ={\t") || (!strings.HasPrefix(path, "/datum/ship_theme/") && !strings.HasPrefix(path, "/datum/ship_upgrade_module/")) {
			continue
		}
		end := len(mask)
		if i+1 < len(lines) {
			end = lines[i+1][0]
		}
		match := assignment.FindSubmatch(mask[line[1]:end])
		if match != nil && withinTypes(string(match[1]), roots) {
			result = append(result, path)
		}
	}
	return result
}

func withinTypes(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func sourcePath(root, path string) (string, error) {
	if filepath.IsAbs(path) {
		var err error
		path, err = filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
	}
	return Inside(root, filepath.ToSlash(path))
}

// Read included sources, including reopened type definitions outside the file
// reported by the parser. Keep include files in the read set for conflict checks.
func removalSources(root, entry string) (map[string][]byte, error) {
	files := map[string][]byte{}
	var visit func(string) error
	visit = func(file string) error {
		file, err := filepath.Abs(file)
		if err != nil {
			return err
		}
		if _, ok := files[file]; ok {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		files[file] = data
		for _, match := range removalInclude.FindAllSubmatch(dmSourceMask(data, false), -1) {
			name := strings.ReplaceAll(string(match[1]), "\\", "/")
			ext := strings.ToLower(filepath.Ext(name))
			if ext != ".dm" && ext != ".dme" {
				continue
			}
			path := includePath(file, name)
			// Inactive conditional includes can point to absent files.
			if _, err := os.Stat(path); os.IsNotExist(err) {
				continue
			}
			if err := visit(path); err != nil {
				return err
			}
		}
		return nil
	}
	return files, visit(entry)
}

// Remove complete absolute type/procedure blocks. A column-zero directive or
// unrelated declaration ends the block; comments and neighboring types survive.
// Reject nested preprocessor edits rather than leaving an unbalanced #if.
func removeDefinitions(data []byte, roots []string) ([]byte, map[string]bool, error) {
	if !containsRemovalRoot(data, roots) {
		return data, map[string]bool{}, nil
	}
	mask := dmSourceMask(data, true)
	removed := map[string]bool{}
	var ranges [][2]int
	start, depth, lastEnd := -1, 0, 0
	for offset := 0; offset < len(mask); {
		end := bytes.IndexByte(mask[offset:], '\n')
		if end < 0 {
			end = len(mask)
		} else {
			end += offset + 1
		}
		line := mask[offset:end]
		trim := bytes.TrimSpace(line)
		top := len(trim) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '\r' && line[0] != '\n'
		if start >= 0 && depth == 0 && top {
			ranges = append(ranges, [2]int{start, lastEnd})
			start = -1
		}
		if start < 0 && top && line[0] == '/' {
			loc := removalPath.FindIndex(line)
			if loc != nil && loc[0] == 0 {
				path := string(line[:loc[1]])
				if withinTypes(path, roots) {
					start = offset
					removed[path] = true
				}
			}
		}
		if start >= 0 {
			if len(trim) > 0 {
				lastEnd = end
			}
			if len(trim) > 0 && trim[0] == '#' {
				return nil, nil, fmt.Errorf("a definition contains conditional code; remove it in source before continuing")
			}
			for _, ch := range line {
				switch ch {
				case '(', '[', '{':
					depth++
				case ')', ']', '}':
					depth--
				}
				if depth < 0 {
					return nil, nil, fmt.Errorf("cannot safely locate the end of a definition")
				}
			}
		}
		offset = end
	}
	if start >= 0 {
		if depth != 0 {
			return nil, nil, fmt.Errorf("cannot remove an incomplete definition")
		}
		ranges = append(ranges, [2]int{start, lastEnd})
	}
	var result []byte
	previous := 0
	for _, span := range ranges {
		result = append(result, data[previous:span[0]]...)
		previous = span[1]
	}
	result = append(result, data[previous:]...)
	return result, removed, nil
}

func referencedType(data []byte, roots []string) string {
	if !containsRemovalRoot(data, roots) {
		return ""
	}
	for _, path := range removalPath.FindAll(dmSourceMask(data, false), -1) {
		if withinTypes(string(path), roots) {
			return string(path)
		}
	}
	return ""
}

func containsRemovalRoot(data []byte, roots []string) bool {
	for _, root := range roots {
		if bytes.Contains(data, []byte(root)) {
			return true
		}
	}
	return false
}

func includePath(file, name string) string {
	path := filepath.FromSlash(strings.ReplaceAll(name, "\\", "/"))
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(file), path)
	}
	return filepath.Clean(path)
}

func removeIncludes(root, file string, data []byte, deleted map[string]bool) []byte {
	mask := dmSourceMask(data, false)
	var output []byte
	previous := 0
	for _, loc := range removalInclude.FindAllSubmatchIndex(mask, -1) {
		name := strings.ReplaceAll(string(data[loc[2]:loc[3]]), "\\", "/")
		path, err := sourcePath(root, includePath(file, name))
		if err == nil && deleted[strings.ToLower(path)] {
			output = append(output, data[previous:loc[0]]...)
			previous = loc[1]
		}
	}
	return append(output, data[previous:]...)
}

func projectMapFiles(root string, extraRoots ...string) ([]string, error) {
	var files []string
	mapRoot, err := Inside(root, "_maps")
	if err != nil {
		return nil, err
	}
	roots := uniqueRemovalPaths(append([]string{mapRoot}, extraRoots...))
	var scanned []string
	for _, mapRoot := range roots {
		covered := false
		for _, prior := range scanned {
			rel, err := filepath.Rel(prior, mapRoot)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		if _, err := sourcePath(root, mapRoot); err != nil {
			return nil, err
		}
		scanned = append(scanned, mapRoot)
		err = filepath.WalkDir(mapRoot, func(path string, entry os.DirEntry, err error) error {
			if os.IsNotExist(err) && path == mapRoot {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("cannot check map references through symlink %s", path)
			}
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".dmm") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, err
}
