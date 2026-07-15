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
	a, e := newApp(root, "secret", time.Second, 1)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "/api/upload?name=a.txt&t=secret", strings.NewReader("hello"))
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
	a, _ := newApp(t.TempDir(), "secret", time.Minute, 1)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/files", nil))
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
}
