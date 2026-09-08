//go:build windows && bundled_previews

package shippreview

import _ "embed"

//go:embed preview-runtime.zip
var bundledRuntime []byte
