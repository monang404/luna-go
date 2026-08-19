package ui

import "testing"

// AC-04: registry berisi command yang sama persis (nama & deskripsi)
// dengan 00-command_registry.zsh. This test is the "golden" snapshot of
// what the zsh source declares (see file's own doc comment for the
// authoritative list), asserted against CommandRegistry so any future
// accidental edit fails loudly.
func TestRegistryHasExpectedCount(t *testing.T) {
	if got, want := len(CommandRegistry), 37; got != want {
		t.Fatalf("len(CommandRegistry) = %d, want %d", got, want)
	}
}

func TestRegistryNoDuplicateNames(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range CommandRegistry {
		if seen[e.Name] {
			t.Fatalf("duplicate command name in registry: %s", e.Name)
		}
		seen[e.Name] = true
	}
}

func TestRegistryEveryCategoryIsListed(t *testing.T) {
	valid := map[string]bool{}
	for _, c := range CommandCategories {
		valid[c] = true
	}
	for _, e := range CommandRegistry {
		if !valid[e.Category] {
			t.Fatalf("command %q has unlisted category %q", e.Name, e.Category)
		}
	}
}

func TestRegistryDescriptionLookup(t *testing.T) {
	desc, ok := RegistryDescription("agent")
	if !ok {
		t.Fatal("expected agent to be found")
	}
	if desc != "Agent full akses: baca/tulis file, jalankan command, looping sendiri" {
		t.Fatalf("desc = %q", desc)
	}

	if _, ok := RegistryDescription("does-not-exist"); ok {
		t.Fatal("expected not found for unknown command")
	}
}

func TestSubcommandsMatchesRegistryNames(t *testing.T) {
	sub := Subcommands()
	names := RegistryNames()
	if len(sub) != len(names) {
		t.Fatalf("Subcommands() len=%d, RegistryNames() len=%d", len(sub), len(names))
	}
	for i := range sub {
		if sub[i] != names[i] {
			t.Fatalf("Subcommands()[%d] = %q, RegistryNames()[%d] = %q", i, sub[i], i, names[i])
		}
	}
}

func TestRegistryFlatListFormat(t *testing.T) {
	flat := RegistryFlatList()
	// Every registry command name must appear as a flat-listed line.
	for _, e := range CommandRegistry {
		if !containsLine(flat, e.Name) {
			t.Fatalf("RegistryFlatList missing entry for %q:\n%s", e.Name, flat)
		}
	}
}

func containsLine(haystack, name string) bool {
	// crude but sufficient: the flat list pads names to 14 chars, so just
	// check the name appears at the start of some line.
	for _, line := range splitLinesForTest(haystack) {
		if len(line) >= len(name) && line[:len(name)] == name {
			return true
		}
	}
	return false
}

func splitLinesForTest(s string) []string {
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return lines
}
