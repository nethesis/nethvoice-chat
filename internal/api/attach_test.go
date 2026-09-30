package api

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFileTransferFromApp(t *testing.T) {
	clear := []byte("OggS pretend voice note")
	key := "EAEA06C77447D04BA82E5D9FD213ED9D"
	k, _ := hex.DecodeString(key)
	block, _ := aes.NewCipher(k)
	enc := make([]byte, len(clear))
	cipher.NewCTR(block, make([]byte, aes.BlockSize)).XORKeyStream(enc, clear)
	mmmsg := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(enc) }))
	defer mmmsg.Close()
	fileClient = mmmsg.Client()

	body := fmt.Sprintf(`{"attachments":[{"content-size":%d,"content-type":"audio/ogg","content-url":"%s/x","encryption-key":"%s","hash":"%d"}]}`, len(enc), mmmsg.URL, key, crc32.ChecksumIEEE(clear))
	f, ok := parseFileTransfer("text/plain", body) // the app does not always say what it sends
	if !ok || len(f.Attachments) != 1 {
		t.Fatalf("not recognised: %v", ok)
	}
	got, err := fetchAttachment(context.Background(), f.Attachments[0])
	if err != nil || string(got) != string(clear) {
		t.Fatalf("decrypt: %q %v", got, err)
	}
	f.Attachments[0].Hash = "12345"
	if _, err := fetchAttachment(context.Background(), f.Attachments[0]); err == nil {
		t.Error("a wrong checksum was accepted")
	}
	if name := fileName(f.Attachments[0]); !strings.HasPrefix(name, "voice-") || !strings.HasSuffix(name, ".ogg") {
		t.Errorf("file name %q", name)
	}
	if _, ok := parseFileTransfer("text/plain", "ciao {\"a\":1}"); ok {
		t.Error("plain text taken for an attachment")
	}
	if _, err := fetchAttachment(context.Background(), attachment{ContentURL: "http://10.0.0.1/x"}); err == nil {
		t.Error("a non-https attachment URL was fetched")
	}
}

func TestFileTransferToApp(t *testing.T) {
	audio := []byte("pretend m4a bytes")
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(audio) }))
	defer up.Close()
	fileClient = up.Client()
	s := &Server{UploadURL: up.URL + "/upload"}
	link := "https://cti.example.com/upload/abc/def/voice-1790757873536.m4a"
	var f acroFile
	if err := json.Unmarshal([]byte(s.fileTransferText(context.Background(), link, "voice-1790757873536.m4a")), &f); err != nil {
		t.Fatal(err)
	}
	a := f.Attachments[0]
	if a.ContentType != "audio/mp4" || a.ContentURL != link || a.ContentSize != int64(len(audio)) || a.Hash != fmt.Sprint(crc32.ChecksumIEEE(audio)) || a.Filename != "" || f.Body != "" {
		t.Errorf("unexpected %+v body %q", a, f.Body)
	}
	if got := s.loopbackUpload(link); got != up.URL+"/upload/abc/def/voice-1790757873536.m4a" {
		t.Errorf("loopback %q", got)
	}
}
