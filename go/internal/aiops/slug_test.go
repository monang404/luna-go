package aiops

import (
	"regexp"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		in     string
		maxLen int
		want   string
	}{
		{"Hello, World!", 0, "hello_world_"},
		{"Buat aplikasi TODO list sederhana", 40, "buat_aplikasi_todo_list_sederhana"},
		{"already_lower", 0, "already_lower"},
	}
	for _, c := range cases {
		if got := Slugify(c.in, c.maxLen); got != c.want {
			t.Errorf("Slugify(%q, %d) = %q, want %q", c.in, c.maxLen, got, c.want)
		}
	}
}

func TestSlugify_Truncates(t *testing.T) {
	long := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	got := Slugify(long, 10)
	if len(got) != 10 {
		t.Errorf("expected truncation to 10 chars, got %d: %q", len(got), got)
	}
}

var tsRE = regexp.MustCompile(`^\d{8}_\d{6}_[0-9a-f]{4}$`)

func TestTimestamp_Format(t *testing.T) {
	ts := Timestamp()
	if !tsRE.MatchString(ts) {
		t.Errorf("Timestamp() = %q, does not match expected format", ts)
	}
}

func TestBackupPath(t *testing.T) {
	p := BackupPath("/tmp/foo.txt")
	if !regexp.MustCompile(`^/tmp/foo\.txt\.bak\.\d{8}_\d{6}_[0-9a-f]{4}$`).MatchString(p) {
		t.Errorf("BackupPath = %q, unexpected format", p)
	}
}
