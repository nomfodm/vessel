package auth

import (
	"testing"
	"time"
)

func TestJWTExp(t *testing.T) {
	// payload {"exp":1893456000,"sub":"x"} → 2030-01-01 UTC
	token := "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOiAxODkzNDU2MDAwLCAic3ViIjogIngifQ.sig"
	got := jwtExp(token)
	if want := time.Unix(1893456000, 0); !got.Equal(want) {
		t.Errorf("jwtExp = %v, want %v", got, want)
	}
}

func TestJWTExpFallback(t *testing.T) {
	before := time.Now()
	for _, bad := range []string{"", "not.a.jwt.x", "onlyonepart", "a.b"} {
		got := jwtExp(bad)
		// fallback is ~15 min from now; just assert it's in the future.
		if !got.After(before) {
			t.Errorf("jwtExp(%q) = %v, want a future fallback", bad, got)
		}
	}
}
