package templateutil

import (
	"fmt"
	"html/template"
)

// Merge takes the the src templates, and merges them in to the dst template.
// Can be used to apply a common layout.e
func Merge(dst *template.Template, src *template.Template) error {
	for _, it := range src.Templates() {
		var err error
		dst, err = dst.AddParseTree(it.Name(), it.Tree)
		if err != nil {
			return fmt.Errorf("adding template %s: %w", it.Name(), err)
		}
	}
	return nil
}
