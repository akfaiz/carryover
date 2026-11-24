package kit

// CoverSource says where a cover comes from (first non-empty wins).
type CoverSource struct {
	Path    string
	Version int64
	URL     string
	Name    string // placeholder initial
	Contain bool   // show the whole image instead of filling (logos, icons)
}
