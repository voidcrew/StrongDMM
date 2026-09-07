package dmenv

import (
	"fmt"
	"sdmm/internal/dmapi/dmvars"
	"strings"
)

// AddDraftType makes a newly authored map type available before the next DME reload.
func (d *Dme) AddDraftType(path string, values map[string]string) error {
	if d.Objects[path] != nil {
		return nil
	}
	parent := d.Objects[path[:strings.LastIndex(path, "/")]]
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
