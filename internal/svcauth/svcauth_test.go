package svcauth

import (
	"testing"
	"time"
)

func TestServicePassword(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	p := Password("s3cret", "mario", "cti.example.com", now)
	if !IsService(p) {
		t.Fatal("not recognised as a service password")
	}
	cases := []struct {
		name               string
		secret, user, host string
		at                 time.Time
		want               bool
	}{
		{"valid", "s3cret", "mario", "cti.example.com", now.Add(10 * time.Second), true},
		{"other user", "s3cret", "luigi", "cti.example.com", now, false},
		{"other host", "s3cret", "mario", "other.example.com", now, false},
		{"wrong secret", "nope", "mario", "cti.example.com", now, false},
		{"empty secret", "", "mario", "cti.example.com", now, false},
		{"expired", "s3cret", "mario", "cti.example.com", now.Add(2 * time.Minute), false},
		{"from the future", "s3cret", "mario", "cti.example.com", now.Add(-2 * time.Minute), false},
	}
	for _, c := range cases {
		if got := Valid(c.secret, c.user, c.host, p, c.at); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	for _, bad := range []string{"", "svc1:", "svc1:abc:def", "svc1:1800000000", "eyJhbGciOi.jwt.token"} {
		if Valid("s3cret", "mario", "cti.example.com", bad, now) {
			t.Errorf("%q accepted", bad)
		}
	}
}
