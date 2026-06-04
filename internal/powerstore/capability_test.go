package powerstore

import "testing"

func TestBulkCapableByVersion(t *testing.T) {
	cases := map[string]bool{"4.1.0.0": true, "4.0.0.0": false, "3.6.0.0": false, "5.0.0.0": true, "": false}
	for ver, want := range cases {
		if got := bulkCapableFromVersion(ver); got != want {
			t.Fatalf("version %q: got %v want %v", ver, got, want)
		}
	}
}
