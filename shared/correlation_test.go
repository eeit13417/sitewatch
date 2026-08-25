package shared

import "testing"

func TestNewCorrelationID_FormatAndUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewCorrelationID()
		if len(id) != 16 {
			t.Fatalf("expected a 16-char hex string, got %q (len %d)", id, len(id))
		}
		for _, r := range id {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
				t.Fatalf("expected lowercase hex, got %q", id)
			}
		}
		if seen[id] {
			t.Fatalf("got a duplicate id after %d generations: %q", i, id)
		}
		seen[id] = true
	}
}
