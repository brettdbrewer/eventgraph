package modelconfig

import "fmt"

// ValidateCapabilities checks that entry satisfies all required capabilities.
// Returns missing capabilities (empty = all satisfied).
func ValidateCapabilities(entry ModelCatalogEntry, required []Capability) []Capability {
	has := make(map[Capability]bool, len(entry.Capabilities))
	for _, c := range entry.Capabilities {
		has[c] = true
	}
	var missing []Capability
	for _, c := range required {
		if !has[c] {
			missing = append(missing, c)
		}
	}
	return missing
}

// ValidateForOperate checks if a model supports the Operate interface.
func ValidateForOperate(entry ModelCatalogEntry) error {
	for _, c := range entry.Capabilities {
		if c == CapOperate {
			return nil
		}
	}
	return fmt.Errorf("model %s does not support operate capability", entry.ID)
}
