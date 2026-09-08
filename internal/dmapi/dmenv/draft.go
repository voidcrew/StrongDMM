package dmenv

import (
	"fmt"
	"sdmm/internal/dmapi/dmvars"
	"strings"
)

// AddDraftType makes a newly authored map type available before the next DME reload.
func (d *Dme) AddDraftType(path string, values map[string]string) error {
	separator := strings.LastIndex(path, "/")
	if !strings.HasPrefix(path, "/") || separator <= 0 || separator == len(path)-1 {
		return fmt.Errorf("invalid draft type path %q", path)
	}
	if d.Objects[path] != nil {
		return nil
	}
	parent := d.Objects[path[:separator]]
	if parent == nil {
		return fmt.Errorf("missing parent for %s", path)
	}
	vars := dmvars.MutableVariables{}
	for k, v := range values {
		vars.Put(k, v)
	}
	obj := &Object{env: d, parent: parent, Path: path, Vars: vars.ToImmutable(), VarFlags: map[string]VarFlags{}}
	obj.Vars.LinkParent(parent.Vars)
	d.Objects[path] = obj
	parent.DirectChildren = append(parent.DirectChildren, path)
	return nil
}
