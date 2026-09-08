package ship

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"sdmm/internal/dmapi/dmenv"
)

type RemovalSpec struct {
	Name       string
	Types      []string
	Helpers    []string
	Maps       []string
	SharedMaps []string
	Metadata   []string
	Draft      bool
	ModuleDir  string
}

// SnapshotRemovalEnvironment freezes object fields for a background preview.
// Variables are immutable; no parent/child traversal is needed by removal.
func SnapshotRemovalEnvironment(dme *dmenv.Dme) *dmenv.Dme {
	copy := *dme
	copy.Objects = make(map[string]*dmenv.Object, len(dme.Objects))
	for path, obj := range dme.Objects {
		one := *obj
		one.DirectChildren = nil
		copy.Objects[path] = &one
	}
	return &copy
}

type RemovalPlan struct {
	Name     string
	Root     string
	Changes  []FileChange
	Types    []string
	Kept     []string
	watched  map[string][32]byte
	maps     []string
	mapRoots []string
}

func (p *RemovalPlan) RemovesType(path string) bool { return withinTypes(path, p.Types) }

// PlanRemoval describes exact file changes without writing the project.
// Helpers and maps still referenced elsewhere remain available to other maps.
func PlanRemoval(dme *dmenv.Dme, spec RemovalSpec) (*RemovalPlan, error) {
	if dme == nil || len(spec.Types) == 0 {
		return nil, fmt.Errorf("choose an entry to remove")
	}
	for _, path := range append(append([]string{}, spec.Types...), spec.Helpers...) {
		if path == "" || !strings.HasPrefix(path, "/") || strings.Count(path, "/") < 3 {
			return nil, fmt.Errorf("invalid removal type %q", path)
		}
	}
	root := dme.RootDir
	sources, err := removalSources(root, dme.RootFile)
	if err != nil {
		return nil, err
	}
	// Newly saved workshop registrations may not be in the loaded parser yet.
	for _, data := range sources {
		spec.Types = append(spec.Types, ownedRegistrationTypes(data, spec.Types)...)
	}
	spec.Types = uniqueRemovalPaths(spec.Types)
	if spec.ModuleDir != "" {
		for _, data := range sources {
			files, err := ownedModuleMaps(data, spec.Types, spec.ModuleDir)
			if err != nil {
				return nil, err
			}
			spec.Maps = append(spec.Maps, files...)
		}
	}
	p := &RemovalPlan{Name: spec.Name, Root: root, Types: append([]string{}, spec.Types...), watched: map[string][32]byte{}}
	remaining := map[string][]byte{}
	found := map[string]bool{}
	for file, data := range sources {
		p.watched[file] = sha256.Sum256(data)
		after, removed, err := removeDefinitions(data, spec.Types)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		for path := range removed {
			found[path] = true
		}
		remaining[file] = after
	}
	for _, path := range spec.Types {
		if !spec.Draft && !found[path] && dme.Objects[path] != nil && dme.Objects[path].Location.File != "" {
			return nil, fmt.Errorf("cannot locate the complete definition of %s; reload the project before removing it", path)
		}
	}
	for file, data := range remaining {
		if path := referencedType(data, spec.Types); path != "" {
			rel, _ := filepath.Rel(root, file)
			return nil, fmt.Errorf("%s still refers to %s; remove that dependency before deleting this entry", filepath.ToSlash(rel), path)
		}
	}
	for _, path := range spec.Maps {
		path, err := sourcePath(root, path)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(filepath.Ext(path), ".dmm") {
			return nil, fmt.Errorf("invalid map path %s", path)
		}
		p.mapRoots = append(p.mapRoots, filepath.Dir(path))
	}
	shared := map[string]bool{}
	literals := map[string]bool{}
	quoted := regexp.MustCompile(`"[^"\r\n]*\.dmm"`)
	for _, data := range remaining {
		if !bytes.Contains(data, []byte(".dmm")) {
			continue
		}
		for _, literal := range quoted.FindAll(dmSourceMask(data, false), -1) {
			value := strings.Trim(string(literal), `"`)
			value = strings.ReplaceAll(value, "\\", "/")
			literals[strings.ToLower(filepath.ToSlash(filepath.Clean(value)))] = true
		}
	}
	for _, file := range spec.SharedMaps {
		if path, err := sourcePath(root, file); err == nil {
			shared[strings.ToLower(path)] = true
			p.mapRoots = append(p.mapRoots, filepath.Dir(path))
		}
	}
	for path, obj := range dme.Objects {
		if withinTypes(path, spec.Types) {
			continue
		}
		for _, file := range registeredMapPaths(root, path, obj) {
			shared[strings.ToLower(file)] = true
			p.mapRoots = append(p.mapRoots, filepath.Dir(file))
		}
	}
	deleted := map[string]bool{}
	addDelete := func(path string) error {
		path, err := sourcePath(root, path)
		if err != nil {
			return err
		}
		key := strings.ToLower(path)
		if deleted[key] {
			return nil
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to remove non-file %s", path)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		p.Changes = append(p.Changes, FileChange{Path: path, Before: before, Existed: true, Delete: true})
		deleted[key] = true
		return nil
	}
	for _, file := range spec.Maps {
		path, err := sourcePath(root, file)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(root, path)
		if literals[strings.ToLower(filepath.ToSlash(rel))] {
			shared[strings.ToLower(path)] = true
		}
		if shared[strings.ToLower(path)] {
			p.Kept = append(p.Kept, path)
		} else if err := addDelete(path); err != nil {
			return nil, err
		}
	}
	for _, file := range spec.Metadata {
		if err := addDelete(file); err != nil {
			return nil, err
		}
	}
	// Check surviving map contents before pruning ship-specific area, port, and
	// outfit definitions. A map outside the selected fleet can still use them.
	p.maps, err = projectMapFiles(root, p.mapRoots...)
	if err != nil {
		return nil, err
	}
	helperKept := map[string]bool{}
	for _, file := range p.maps {
		if deleted[strings.ToLower(file)] {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		if path := referencedType(data, spec.Types); path != "" {
			return nil, fmt.Errorf("%s still refers to %s", file, path)
		}
		p.watched[file] = sha256.Sum256(data)
		for _, helper := range spec.Helpers {
			if referencedType(data, []string{helper}) != "" {
				helperKept[helper] = true
			}
		}
	}
	// A helper can refer to another helper. Iterate retention until dependencies
	// of every retained helper are retained too.
	for {
		var remove []string
		for _, helper := range spec.Helpers {
			if !helperKept[helper] {
				remove = append(remove, helper)
			}
		}
		changed := false
		for _, data := range remaining {
			after, _, err := removeDefinitions(data, remove)
			if err != nil {
				return nil, err
			}
			for _, helper := range remove {
				if referencedType(after, []string{helper}) != "" {
					helperKept[helper] = true
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	var helpers []string
	for _, helper := range spec.Helpers {
		if helperKept[helper] {
			p.Kept = append(p.Kept, helper)
		} else {
			helpers = append(helpers, helper)
			p.Types = append(p.Types, helper)
		}
	}
	for file, data := range remaining {
		after, _, err := removeDefinitions(data, helpers)
		if err != nil {
			return nil, err
		}
		remaining[file] = after
		if !bytes.Equal(sources[file], after) && len(bytes.TrimSpace(dmSourceMask(after, true))) == 0 && file != dme.RootFile {
			if err := addDelete(file); err != nil {
				return nil, err
			}
		}
	}
	for file, data := range remaining {
		if deleted[strings.ToLower(file)] {
			continue
		}
		after := removeIncludes(root, file, data, deleted)
		if !bytes.Equal(sources[file], after) {
			if _, err := sourcePath(root, file); err != nil {
				return nil, fmt.Errorf("removal would change a source outside this project: %s", file)
			}
			p.Changes = append(p.Changes, FileChange{Path: file, Before: sources[file], After: after, Existed: true})
		}
	}
	sort.Slice(p.Changes, func(i, j int) bool { return p.Changes[i].Path < p.Changes[j].Path })
	sort.Strings(p.Types)
	sort.Strings(p.Kept)
	return p, nil
}

func registeredMapPaths(root, path string, obj *dmenv.Object) []string {
	var relative []string
	v := obj.Vars
	if strings.HasPrefix(path, "/datum/map_template/ruin/") {
		if suffix := text(v, "suffix"); suffix != "" {
			relative = append(relative, text(v, "prefix")+suffix)
		}
	} else if strings.HasPrefix(path, "/datum/map_template/shuttle/") {
		if suffix := text(v, "suffix"); suffix != "" {
			relative = append(relative, text(v, "prefix")+text(v, "port_id")+"_"+suffix+".dmm")
		}
	}
	for _, field := range []string{"mappath", "map_path", "map_file"} {
		if file := text(v, field); strings.HasPrefix(strings.ReplaceAll(file, "\\", "/"), "_maps/") {
			relative = append(relative, file)
		}
	}
	var result []string
	for _, file := range relative {
		if resolved, err := Inside(root, strings.ReplaceAll(file, "\\", "/")); err == nil {
			result = append(result, resolved)
		}
	}
	return result
}

func (p *RemovalPlan) Apply(recoveryDir string) (string, error) {
	for path, before := range p.watched {
		data, err := os.ReadFile(path)
		if err != nil || sha256.Sum256(data) != before {
			return "", fmt.Errorf("project files changed since this removal was reviewed; review the removal again")
		}
	}
	maps, err := projectMapFiles(p.Root, p.mapRoots...)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(maps, p.maps) {
		return "", fmt.Errorf("the project's maps changed; review the removal again")
	}
	for _, change := range p.Changes {
		if err := change.unchanged(); err != nil {
			return "", err
		}
	}
	if len(p.Changes) == 0 {
		return "", nil
	}
	if recoveryDir == "" {
		return "", fmt.Errorf("a recovery folder is required")
	}
	if err := os.MkdirAll(recoveryDir, 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(recoveryDir, "removed-"+time.Now().Format("20060102-150405")+"-*.zip")
	if err != nil {
		return "", fmt.Errorf("create recovery copy: %w", err)
	}
	archive := zip.NewWriter(file)
	for _, change := range p.Changes {
		rel, err := filepath.Rel(p.Root, change.Path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			archive.Close()
			file.Close()
			return "", fmt.Errorf("recovery path escapes project")
		}
		entry, err := archive.Create(filepath.ToSlash(rel))
		if err == nil {
			_, err = entry.Write(change.Before)
		}
		if err != nil {
			archive.Close()
			file.Close()
			return "", err
		}
	}
	manifest, _ := json.MarshalIndent(struct {
		Name, Project string
		RemovedAt     time.Time
	}{p.Name, p.Root, time.Now()}, "", "  ")
	entry, err := archive.Create("REMOVAL.json")
	if err == nil {
		_, err = entry.Write(manifest)
	}
	if err == nil {
		err = archive.Close()
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := WriteChanges(p.Root, p.Changes); err != nil {
		return file.Name(), err
	}
	return file.Name(), nil
}
