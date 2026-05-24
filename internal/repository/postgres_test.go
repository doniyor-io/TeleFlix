package repository

import "testing"

func TestNormalizeStoredChannelID(t *testing.T) {
	if got := normalizeStoredChannelID(1234567890); got != -1001234567890 {
		t.Fatalf("expected normalized supergroup/channel id, got %d", got)
	}
	if got := normalizeStoredChannelID(-1001234567890); got != -1001234567890 {
		t.Fatalf("expected existing negative id unchanged, got %d", got)
	}
}
