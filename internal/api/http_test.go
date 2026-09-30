package api

import (
	"encoding/base64"
	"testing"
)

func TestPushEndpoint(t *testing.T) {
	for ep, want := range map[string]bool{
		"https://fcm.googleapis.com/fcm/send/abc":                true,
		"https://updates.push.services.mozilla.com/wpush/v2/abc": true,
		"https://web.push.apple.com/QGx":                         true,
		"https://wns2-db5p.notify.windows.com/w/?token=x":        true,
		"http://fcm.googleapis.com/fcm/send/abc":                 false,
		"https://fcm.googleapis.com:8443/x":                      false,
		"https://10.0.0.5/x":                                     false,
		"https://evilpush.apple.com.attacker.net/x":              false,
		"https://notpush.services.mozilla.com.evil/x":            false,
		"https://push.apple.com.evil/x":                          false,
		"javascript:alert(1)":                                    false,
	} {
		if got := pushEndpoint(ep); got != want {
			t.Errorf("%s: got %v, want %v", ep, got, want)
		}
	}
}

func TestArchiveIDs(t *testing.T) {
	for id, want := range map[string]bool{"1790707806038396": true, "": false, "18a3f": false, "123456789012345678901": false} {
		if archiveID(id) != want {
			t.Errorf("archiveID(%q) != %v", id, want)
		}
	}
	if !olderID("999999999999999", "1790707806038396") || olderID("1790707806038397", "1790707806038396") {
		t.Error("olderID compares as strings")
	}
}

func TestThrottle(t *testing.T) {
	s := &Server{}
	for i := 0; i < 5; i++ {
		if s.throttled("mario") {
			t.Fatalf("throttled after %d failures", i)
		}
		s.failed("mario")
	}
	if !s.throttled("mario") {
		t.Error("not throttled after five failures")
	}
	if s.throttled("luigi") {
		t.Error("another account was throttled")
	}
}

func TestTokenLogin(t *testing.T) {
	enc := func(claims string) string {
		return "eyJhbGciOiJIUzI1NiJ9." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".c2lnbmF0dXJlLWJ5dGVz"
	}
	for p, want := range map[string]bool{
		enc(`{"id":"mario"}`):        true,
		"NethVoice,4321":             false,
		"a.b.c":                      false,
		"with space.in the.password": false,
	} {
		if looksLikeJWT(p) != want {
			t.Errorf("looksLikeJWT(%q) != %v", p, want)
		}
	}
	if !needsOTP(enc(`{"2fa":true,"otp_verified":false}`)) || !needsOTP(enc(`{"2fa":true}`)) {
		t.Error("a 2FA token without the OTP should need it")
	}
	if needsOTP(enc(`{"2fa":true,"otp_verified":true}`)) || needsOTP(enc(`{"2fa":false}`)) || needsOTP("garbage") {
		t.Error("needsOTP too eager")
	}
}

func TestEmojify(t *testing.T) {
	for in, want := range map[string]string{
		":+1:":                        "👍",
		"great :tada: :joy:":          "great 🎉 😂",
		"at 10:30:00 ok":              "at 10:30:00 ok",
		":not_a_code: stays":          ":not_a_code: stays",
		"`:+1:` in code":              "`:+1:` in code",
		"```\n:+1:\n``` then :heart:": "```\n:+1:\n``` then ❤️",
		"plain text":                  "plain text",
	} {
		if got := emojify(in); got != want {
			t.Errorf("emojify(%q) = %q, want %q", in, got, want)
		}
	}
}
