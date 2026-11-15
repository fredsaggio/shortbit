package handlers

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateKeepsShortTextAndCutsOnCharacterBoundaries(t *testing.T) {
	if got := truncate("access_denied", 64); got != "access_denied" {
		t.Errorf("truncate(short) = %q", got)
	}

	long := strings.Repeat("é", 100)
	got := truncate(long, 64)
	if !utf8.ValidString(got) || !strings.HasSuffix(got, "…") || len(got) > 64+len("…") {
		t.Errorf("truncate(long) = %q (%d bytes), want valid UTF-8 cut near 64 bytes", got, len(got))
	}
}
