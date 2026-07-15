package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArtifactEndpointRejectsPublicAndMissingToken(t *testing.T) {
	if _, _, e := artifactEndpoint("http://8.8.8.8:1234/qqq?t=x"); e == nil {
		t.Fatal("public IP accepted")
	}
	if _, _, e := artifactEndpoint("http://192.168.0.2:1234/qqq"); e == nil {
		t.Fatal("missing token accepted")
	}
	if _, _, e := artifactEndpoint("http://192.168.0.2:1234/qqq?t=x"); e != nil {
		t.Fatal(e)
	}
}

func TestCleanName(t *testing.T) {
	if got := cleanName(`../bad:name.txt`); got != "bad_name.txt" {
		t.Fatalf("got %q", got)
	}
}
func TestUploadExpire(t *testing.T) {
	root := t.TempDir()
	a, e := newApp(root, "owner", "secret", time.Second, 1)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "/api/upload?name=a.txt&t=owner", strings.NewReader("hello"))
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("upload %d %s", w.Code, w.Body.String())
	}
	if len(a.list()) != 1 {
		t.Fatal("file not active")
	}
	if len(a.list()[0].SHA256) != 64 {
		t.Fatal("sha256 missing")
	}
	time.Sleep(1100 * time.Millisecond)
	a.expire()
	if len(a.list()) != 0 {
		t.Fatal("file still active")
	}
	if _, e = os.Stat(filepath.Join(root, "Expired", "a.txt")); e != nil {
		t.Fatal(e)
	}
}
func TestRejectsMissingToken(t *testing.T) {
	a, _ := newApp(t.TempDir(), "owner", "secret", time.Minute, 1)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/files", nil))
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestReaderCannotUploadAndOwnerCanRevoke(t *testing.T) {
	a, _ := newApp(t.TempDir(), "owner", "reader", time.Minute, 1)
	h := a.handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/upload?name=a.txt&t=reader", strings.NewReader("x")))
	if w.Code != 403 {
		t.Fatalf("reader upload got %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/upload?name=a.txt&t=owner", strings.NewReader("x")))
	if w.Code != 201 {
		t.Fatalf("owner upload got %d", w.Code)
	}
	id := a.list()[0].ID
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/files/"+id+"?t=owner", nil))
	if w.Code != 200 || len(a.list()) != 0 {
		t.Fatalf("revoke got %d", w.Code)
	}
}
