package main

import (
	"testing"
	"time"

	"github.com/nethesis/nethvoice-chat/internal/svcauth"
)

func TestHandle(t *testing.T) {
	secret = "s3cret"
	now := time.Unix(1_800_000_000, 0)
	who := func(tok string) string {
		if tok == "jwt-of-mario" {
			return "mario"
		}
		return ""
	}
	svc := svcauth.Password("s3cret", "mario", "cti.example.com", now)
	cases := []struct {
		req  string
		want bool
	}{
		{"auth:mario:cti.example.com:jwt-of-mario", true},
		{"auth:luigi:cti.example.com:jwt-of-mario", false}, // a token only logs in its owner
		{"auth:mario:cti.example.com:", false},
		{"auth:mario:cti.example.com", false},
		{"auth:Mario!:cti.example.com:jwt-of-mario", false},
		{"auth:mario:cti.example.com:" + svc, true},
		{"auth:luigi:cti.example.com:" + svc, false},
		{"auth:mario:other.example.com:" + svc, false},
		{"isuser:mario:cti.example.com", true},
		{"isuser:../etc:cti.example.com", false},
		{"setpass:mario:cti.example.com:x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := handle(c.req, who, now); got != c.want {
			t.Errorf("%q: got %v, want %v", c.req, got, c.want)
		}
	}
}
