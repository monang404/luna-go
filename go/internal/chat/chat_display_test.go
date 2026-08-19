package chat

import "testing"

func TestSplitReply_MarkerContract(t *testing.T) {
	raw := "Analisis: blah\nRencana: blah\n@@JAWABAN@@\nIni jawaban final."
	thought, answer := SplitReply(raw)
	if thought != "Analisis: blah\nRencana: blah" {
		t.Errorf("thought = %q", thought)
	}
	if answer != "Ini jawaban final." {
		t.Errorf("answer = %q", answer)
	}
}

func TestSplitReply_NoMarker_NoFallback(t *testing.T) {
	raw := "Cuma jawaban langsung, gak ada marker apa pun."
	thought, answer := SplitReply(raw)
	if thought != "" {
		t.Errorf("expected empty thought, got %q", thought)
	}
	if answer != raw {
		t.Errorf("expected full raw text as answer, got %q", answer)
	}
}

func TestSplitReply_ThoughtHeadingFallback(t *testing.T) {
	raw := "Jawaban duluan di sini.\n**Thought**\nReasoning nempel di belakang."
	thought, answer := SplitReply(raw)
	if answer != "Jawaban duluan di sini." {
		t.Errorf("answer = %q", answer)
	}
	if thought != "Reasoning nempel di belakang." {
		t.Errorf("thought = %q", thought)
	}
}

func TestSplitReply_EmptyMarkerContent_FallsBackToRaw(t *testing.T) {
	raw := "@@JAWABAN@@"
	_, answer := SplitReply(raw)
	if answer != raw {
		t.Errorf("expected raw fallback when marker produces an empty answer, got %q", answer)
	}
}
