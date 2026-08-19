package components

import "testing"

func TestProgressWithMessage(t *testing.T) {
	got := Progress(4, 7, "Compiling", testMode(true, 40))
	want := "● Compiling  [4/7]  ███████████░░░░░░░░░\n"
	if got != want {
		t.Fatalf("Progress = %q, want %q", got, want)
	}
}

func TestProgressAsciiNoMessage(t *testing.T) {
	got := Progress(2, 4, "", testMode(false, 40))
	want := "[2/4]  ##########..........\n"
	if got != want {
		t.Fatalf("Progress = %q, want %q", got, want)
	}
}

func TestProgressClampsBounds(t *testing.T) {
	got := Progress(-5, 0, "", testMode(false, 40))
	// total<=0 -> 1; current<0 -> 0
	want := "[0/1]  ....................\n"
	if got != want {
		t.Fatalf("Progress = %q, want %q", got, want)
	}

	got2 := Progress(99, 10, "", testMode(false, 40))
	want2 := "[10/10]  ####################\n"
	if got2 != want2 {
		t.Fatalf("Progress = %q, want %q", got2, want2)
	}
}
