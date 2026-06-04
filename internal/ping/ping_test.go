package ping

import (
	"context"
	"testing"
)

// A closed port must degrade to offline without an error — that contract is what
// the UI relies on.
func TestPingOffline(t *testing.T) {
	st := New(nil).Ping(context.Background(), "127.0.0.1", 1)
	if st.Online {
		t.Fatal("expected offline for closed port")
	}
}
