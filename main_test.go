package main

import (
	"encoding/json"
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
	a, e := newApp(root, "owner", "secret", "test", time.Second, 1, false)
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
	a, _ := newApp(t.TempDir(), "owner", "secret", "test", time.Minute, 1, false)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/files", nil))
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestReaderCannotUploadAndOwnerCanRevoke(t *testing.T) {
	a, _ := newApp(t.TempDir(), "owner", "reader", "test", time.Minute, 1, false)
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

func TestOneTimeDownloadIsConsumed(t *testing.T) {
	a, _ := newApp(t.TempDir(), "owner", "reader", "once-test", time.Minute, 1, true)
	h := a.handler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/upload?name=once.txt&t=owner", strings.NewReader("one")))
	if w.Code != 201 {
		t.Fatalf("upload got %d", w.Code)
	}
	id := a.list()[0].ID
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/download/"+id+"?t=reader", nil))
	if w.Code != 200 || w.Body.String() != "one" {
		t.Fatalf("download got %d %q", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/download/"+id+"?t=reader", nil))
	if w.Code != 404 {
		t.Fatalf("second download got %d", w.Code)
	}
}

func TestStructuredAPIError(t *testing.T) {
	a, _ := newApp(t.TempDir(), "owner", "reader", "test", time.Minute, 1, false)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/files?t=bad", nil))
	if w.Code != 403 || !strings.Contains(w.Body.String(), `"schema":"qqqshare-error/v1"`) || !strings.Contains(w.Body.String(), `"code":"invalid_token"`) {
		t.Fatalf("unexpected error: %s", w.Body.String())
	}
}

func TestArtifactManifestUsesConfiguredID(t *testing.T) {
	a, _ := newApp(t.TempDir(), "owner", "reader", "art_configured", time.Minute, 1, false)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/artifact?t=reader", nil))
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var info artifactInfo
	if json.Unmarshal(w.Body.Bytes(), &info) != nil || info.ArtifactID != "art_configured" {
		t.Fatalf("manifest: %s", w.Body.String())
	}
}

func TestRegistryRoundTripAndExpiry(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	entry := registryEntry{Schema: "qqqshare-registry/v1", ArtifactID: "art_test", URL: "http://192.168.0.2/qqq?t=reader", OwnerURL: "http://127.0.0.1:1/qqq?t=owner", Scope: "lan", CreatedAt: 1, ExpiresAt: time.Now().Add(-time.Second).UnixMilli(), Status: "active"}
	if e := saveRegistryEntry(entry); e != nil {
		t.Fatal(e)
	}
	got, ok := loadRegistryEntry("art_test")
	if !ok || got.OwnerURL != entry.OwnerURL {
		t.Fatalf("registry round trip: %#v", got)
	}
	items := registryEntries()
	if len(items) != 1 || items[0].Status != "expired" {
		t.Fatalf("registry entries: %#v", items)
	}
}
