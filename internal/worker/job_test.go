package worker

import "testing"

func TestCreateETagUsesContent(t *testing.T) {
	first := createETag([]byte("catalog"))
	second := createETag([]byte("catalog"))
	changed := createETag([]byte("changed"))

	if first != second {
		t.Fatalf("equal content produced different ETags: %q and %q", first, second)
	}
	if first == changed {
		t.Fatalf("different content produced the same ETag: %q", first)
	}
	if len(first) < 4 || first[:3] != `W/"` {
		t.Fatalf("ETag = %q, want a weak quoted ETag", first)
	}
}
