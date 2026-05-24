package bot

import "testing"

func TestMembershipCheckChannelIDsForPositiveID(t *testing.T) {
	got := membershipCheckChannelIDs(1234567890)
	if len(got) != 2 {
		t.Fatalf("expected 2 candidate ids, got %d", len(got))
	}
	if got[0] != 1234567890 {
		t.Fatalf("expected original id first, got %d", got[0])
	}
	if got[1] != -1001234567890 {
		t.Fatalf("expected -100 fallback id, got %d", got[1])
	}
}

func TestMembershipCheckChannelIDsForNegativeID(t *testing.T) {
	got := membershipCheckChannelIDs(-1001234567890)
	if len(got) != 1 || got[0] != -1001234567890 {
		t.Fatalf("expected original negative id only, got %+v", got)
	}
}

func TestParseChannelIDString(t *testing.T) {
	got, err := parseChannelID("-1001234567890")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if got != -1001234567890 {
		t.Fatalf("expected parsed channel id, got %d", got)
	}
}
