package ship

import (
	"bytes"
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"strings"
)

type FileChange struct {
	Path          string
	Before, After []byte
	Existed       bool
	Delete        bool
}

func (c FileChange) unchanged() error {
	data, err := os.ReadFile(c.Path)
	if !c.Existed && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot check %s: %w", c.Path, err)
	}
	if !c.Existed || !bytes.Equal(data, c.Before) {
		return fmt.Errorf("%s changed outside this project; save stopped", c.Path)
	}
	return nil
}

// WriteChanges stages every file before replacing any destination, checks for
// external edits, and rolls back replacements if a filesystem operation fails.
func WriteChanges(root string, changes []FileChange) error {
	return writeChanges(root, changes, os.Rename)
}

func writeChanges(root string, changes []FileChange, rename func(string, string) error) error {
	type staged struct {
		change         FileChange
		temp, backup   string
		moved, written bool
	}
	files := make([]staged, 0, len(changes))
	seen := map[string]bool{}
	defer func() {
		for _, f := range files {
			if f.temp != "" {
				_ = os.Remove(f.temp)
			}
		}
	}()
	for _, c := range changes {
		rel, err := filepath.Rel(root, c.Path)
		if err != nil {
			return err
		}
		path, err := Inside(root, rel)
		if err != nil {
			return err
		}
		key := strings.ToLower(filepath.Clean(path))
		if seen[key] {
			return fmt.Errorf("duplicate save destination %s", path)
		}
		seen[key] = true
		c.Path = path
		if err := c.unchanged(); err != nil {
			return err
		}
		if c.Delete && (!c.Existed || len(c.After) != 0) {
			return fmt.Errorf("invalid deletion: %s", path)
		}
		if !c.Delete && c.Existed && bytes.Equal(c.Before, c.After) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		temp, err := os.CreateTemp(filepath.Dir(path), ".ship-save-*")
		if err != nil {
			return err
		}
		files = append(files, staged{change: c, temp: temp.Name(), backup: temp.Name() + ".previous"})
		if _, err = temp.Write(c.After); err == nil {
			err = temp.Sync()
		}
		closeErr := temp.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		mode := os.FileMode(0644)
		if c.Existed {
			if info, e := os.Stat(path); e == nil {
				mode = info.Mode().Perm()
			}
		}
		if err := os.Chmod(temp.Name(), mode); err != nil {
			return err
		}
	}
	for _, f := range files {
		if err := f.change.unchanged(); err != nil {
			return err
		}
	}
	rollback := func(cause error) error {
		var failures []string
		for i := len(files) - 1; i >= 0; i-- {
			f := &files[i]
			if f.written {
				data, err := os.ReadFile(f.change.Path)
				if err != nil || !bytes.Equal(data, f.change.After) {
					failures = append(failures, "keep recovery copy "+f.backup)
					continue
				}
				if err := os.Remove(f.change.Path); err != nil {
					failures = append(failures, err.Error())
					continue
				}
			}
			if f.moved {
				if err := os.Rename(f.backup, f.change.Path); err != nil {
					failures = append(failures, "recovery copy: "+f.backup+": "+err.Error())
				}
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("%w; rollback needs attention: %s", cause, strings.Join(failures, "; "))
		}
		return cause
	}
	for i := range files {
		f := &files[i]
		if err := f.change.unchanged(); err != nil {
			return rollback(err)
		}
		if f.change.Existed {
			if err := rename(f.change.Path, f.backup); err != nil {
				return rollback(err)
			}
			f.moved = true
		}
		if f.change.Delete {
			continue
		}
		if err := rename(f.temp, f.change.Path); err != nil {
			return rollback(err)
		}
		f.written = true
	}
	for _, f := range files {
		if f.moved {
			if err := os.Remove(f.backup); err != nil {
				log.Warn().Err(err).Msg("Files saved; recovery copy retained at " + f.backup)
			}
		}
	}
	return nil
}
