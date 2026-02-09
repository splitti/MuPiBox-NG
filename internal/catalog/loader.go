package catalog

import (
	"encoding/json"
	"os"
)

func LoadCatalog(path string) (*Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}

	return &c, nil
}
