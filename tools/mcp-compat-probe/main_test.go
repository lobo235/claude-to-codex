package main

import "testing"

func TestDefaultProbesIncludeBridgePrefixedToolCalls(t *testing.T) {
	got := map[string]bool{}
	for _, probe := range probes("/tmp/example.txt") {
		got[probe.name] = true
	}

	for _, name := range []string{
		"everything__get-structured-content",
		"filesystem__list_allowed_directories",
		"filesystem__read_text_file",
		"memory__read_graph",
		"sequential-thinking__sequentialthinking",
		"time__get_current_time",
		"sqlite__list_tables",
	} {
		if !got[name] {
			t.Fatalf("default probes missing %q; got %#v", name, got)
		}
	}
}
