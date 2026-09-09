package ship

// SourceMask blanks DM comments and optionally strings without moving offsets.
func SourceMask(data []byte, strings bool) []byte { return dmSourceMask(data, strings) }

// RewriteLiteralList edits one unambiguous type's literal list, preserving the surrounding source.
func RewriteLiteralList(data []byte, path, field, value string) ([]byte, error) {
	return rewriteLiteralList(data, path, field, value)
}

// AddInclude registers a generated DM source beside the project's other includes.
func AddInclude(data []byte, root, file string) []byte { return addInclude(data, root, file) }
