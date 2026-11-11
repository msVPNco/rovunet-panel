package clientcfg

import (
	_ "embed"
	"encoding/json"
)

//go:embed display_names.json
var approvedDisplayNamesJSON []byte

var approvedDisplayNames = func() map[string]string {
	var names map[string]string
	if err := json.Unmarshal(approvedDisplayNamesJSON, &names); err != nil {
		panic("invalid embedded profile display names")
	}
	return names
}()

// ProfileDisplayName returns an approved presentation label for the existing
// profile ID. It must never be used as a tunnel/core tag, credential seed or
// subscription item ID. Custom IDs retain their configured names.
func ProfileDisplayName(id string) string { return approvedDisplayNames[id] }
