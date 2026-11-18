package components

// LabelOf is a save location's label, or its key when it has none.
func LabelOf(label, key string) string {
	if label != "" {
		return label
	}
	return key
}
