package main

type nameMapper struct {
	counts map[string]int
}

func newNameMapper(names []string) nameMapper {
	counts := make(map[string]int, len(names))
	for _, name := range names {
		counts[name]++
	}
	return nameMapper{counts: counts}
}

func (m nameMapper) exposed(serverName, original string) string {
	if m.counts[original] <= 1 {
		return original
	}
	return serverName + "__" + original
}
