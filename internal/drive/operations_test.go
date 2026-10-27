package drive

import "testing"

func TestIsAllowedFolder(t *testing.T) {
	allowed := []string{"Background", "Featured Image", "Stills", "Dir", "  stills  ", "DIR", "featured image"}
	for _, name := range allowed {
		if !isAllowedFolder(name) {
			t.Errorf("isAllowedFolder(%q) = false, want true", name)
		}
	}
	rejected := []string{"", "Press", "Still", "Directors", "FeaturedImage", "Stills/old"}
	for _, name := range rejected {
		if isAllowedFolder(name) {
			t.Errorf("isAllowedFolder(%q) = true, want false", name)
		}
	}
}
