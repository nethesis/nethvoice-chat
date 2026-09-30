package store

import "testing"

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPNMNodeIsPerAccount(t *testing.T) {
	s := open(t)
	n1, err := s.PutPNM(PNMToken{Node: "pnm:a", Username: "mario", Selector: "sel", Token: "t1"})
	if err != nil || n1 != "pnm:a" {
		t.Fatalf("first report: %q %v", n1, err)
	}
	// The same account reporting again keeps its node and gets the new token.
	n2, _ := s.PutPNM(PNMToken{Node: "pnm:b", Username: "mario", Selector: "sel", Token: "t2"})
	if n2 != "pnm:a" {
		t.Fatalf("same account got a new node %q", n2)
	}
	if tok, _ := s.PNM("pnm:a"); tok == nil || tok.Token != "t2" || tok.Username != "mario" {
		t.Fatalf("token not updated: %+v", tok)
	}
	// Somebody else reporting the same selector gets a node of their own and cannot take mario's.
	n3, _ := s.PutPNM(PNMToken{Node: "pnm:c", Username: "luigi", Selector: "sel", Token: "evil"})
	if n3 != "pnm:c" {
		t.Fatalf("other account got %q", n3)
	}
	if tok, _ := s.PNM("pnm:a"); tok.Username != "mario" || tok.Token != "t2" {
		t.Fatalf("mario's device was taken over: %+v", tok)
	}
}

func TestWebSubChangesNodeWithOwner(t *testing.T) {
	s := open(t)
	w := WebSub{Node: "web:1", Username: "mario", Endpoint: "https://fcm.googleapis.com/x", P256dh: "p", Auth: "a"}
	n1, _ := s.PutWebSub(w)
	w.Node = "web:2"
	if n, _ := s.PutWebSub(w); n != n1 {
		t.Fatalf("same browser and user got a new node %q", n)
	}
	w.Node, w.Username = "web:3", "luigi"
	n3, _ := s.PutWebSub(w)
	if n3 != "web:3" {
		t.Fatalf("new owner kept the old node %q", n3)
	}
	if old, _ := s.WebSub(n1); old != nil {
		t.Fatalf("old node still delivers to the browser: %+v", old)
	}
}

func TestMobileFollowsTheApp(t *testing.T) {
	s := open(t)
	node, _ := s.PutPNM(PNMToken{Node: "pnm:m", Username: "mario", Selector: "sel", Token: "t"})
	users := func() []string { u, _ := s.PNMUsers(); return u }
	if got := users(); len(got) != 1 {
		t.Fatalf("a fresh registration should count: %v", got)
	}
	s.Pushed(node)
	if got := users(); len(got) != 1 {
		t.Fatalf("a push just sent is not yet unanswered: %v", got)
	}
	// Three minutes and more without the app fetching: logged out.
	if _, err := s.db.Exec(`UPDATE pnm_tokens SET pending_since = pending_since - 200`); err != nil {
		t.Fatal(err)
	}
	if got := users(); len(got) != 0 {
		t.Fatalf("an unanswered push should drop the account: %v", got)
	}
	s.Seen("mario")
	if got := users(); len(got) != 1 {
		t.Fatalf("the app answering should bring it back: %v", got)
	}
	// Nothing from the app for more than a week.
	if _, err := s.db.Exec(`UPDATE pnm_tokens SET last_seen = 1, created = 1`); err != nil {
		t.Fatal(err)
	}
	if got := users(); len(got) != 0 {
		t.Fatalf("a phone silent for a week should not count: %v", got)
	}
}
