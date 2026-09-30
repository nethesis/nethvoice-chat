package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nethesis/nethvoice-chat/internal/store"
)

func TestPNMGroupPush(t *testing.T) {
	var got map[string]any
	pnm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer pnm.Close()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutPNM(store.PNMToken{Node: "pnm:n", Username: "edoardo", Selector: "s", Token: "tok", AppID: "app"}); err != nil {
		t.Fatal(err)
	}
	r, err := NewRouter(st, pnm.URL)
	if err != nil {
		t.Fatal(err)
	}
	var askedUser, askedPeer string
	r.Names = func(u string) string { return map[string]string{"mario": "Mario Rossi"}[u] }
	r.RoomName = func(context.Context, string) string { return "Gruppo con Copertina" }
	r.ArchiveID = func(_ context.Context, user, peer string) string {
		askedUser, askedPeer = user, peer
		return "1790707806038396"
	}

	r.Deliver(context.Background(), Notification{Node: "pnm:n", Count: "3", Sender: "g-1@conference.cti.example.com/mario", Body: "ciao"})

	if askedUser != "edoardo" || askedPeer != "g-1@conference.cti.example.com" {
		t.Errorf("archive id asked for %q with %q", askedUser, askedPeer)
	}
	want := map[string]any{
		"Id": "1790707806038396", "UserName": "Gruppo con Copertina", "ThreadId": "Gruppo con Copertina",
		"UserDisplayName": "Gruppo con Copertina", "Message": "Mario Rossi: ciao", "DeviceToken": "tok", "Badge": float64(3),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}

func TestDeliverDropsEmpty(t *testing.T) {
	called := false
	pnm := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer pnm.Close()
	st, _ := store.Open(t.TempDir())
	_, _ = st.PutPNM(store.PNMToken{Node: "pnm:n", Username: "edoardo", Selector: "s", Token: "tok"})
	r, _ := NewRouter(st, pnm.URL)
	r.Deliver(context.Background(), Notification{Node: "pnm:n"})                                                              // a typing indicator
	r.Deliver(context.Background(), Notification{Node: "pnm:unknown", Body: "x", Sender: "a@b"})                              // nobody's node
	r.Deliver(context.Background(), Notification{Node: "pnm:n", Body: "x", Sender: "g-1@conference.cti.example.com/edoardo"}) // edoardo's own
	if called {
		t.Error("pushed something that should have been dropped")
	}
}

func TestPreview(t *testing.T) {
	for in, want := range map[string]string{"voice-1790757873536.m4a": "Voice message", "IMG_0001.JPG": "Photo", "report.pdf": "File: report.pdf", "ciao a tutti.": "ciao a tutti.", "v1.2": "v1.2"} {
		if got := preview(in); got != want {
			t.Errorf("preview(%q) = %q, want %q", in, got, want)
		}
	}
}
