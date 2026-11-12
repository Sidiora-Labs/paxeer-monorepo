package fanout

import "testing"

func TestSmallIntegerAndOptionalTextEncoding(t *testing.T) {
	for _, pair := range []struct {
		value int
		want  string
	}{{0, "0"}, {7, "7"}, {-42, "-42"}, {123456789, "123456789"}} {
		if got := itoa(pair.value); got != pair.want {
			t.Fatalf("integer %d encoded as %q, want %q", pair.value, got, pair.want)
		}
	}
	if value := textOrNull(nil); value.Valid {
		t.Fatalf("nil text was valid: %+v", value)
	}
	empty := ""
	if value := textOrNull(&empty); value.Valid {
		t.Fatalf("empty text was valid: %+v", value)
	}
	content := "committed message"
	if value := textOrNull(&content); !value.Valid || value.String != content {
		t.Fatalf("nonempty text = %+v", value)
	}
}
