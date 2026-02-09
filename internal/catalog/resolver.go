package catalog

import "sort"

func ResolveSource(item Item) *Source {
	if len(item.Sources) == 0 {
		return nil
	}

	sort.Slice(item.Sources, func(i, j int) bool {
		return item.Sources[i].Priority < item.Sources[j].Priority
	})

	for _, src := range item.Sources {
		if sourceAvailable(src) {
			return &src
		}
	}

	return nil
}

func sourceAvailable(src Source) bool {
	switch src.Type {
	case "amazon", "spotify", "rss":
		return true // später: Login / Netz prüfen
	case "local":
		return true // später: Pfad existiert?
	default:
		return false
	}
}
