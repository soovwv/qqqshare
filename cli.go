package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

type artifactInfo struct {
	Schema     string  `json:"schema"`
	ArtifactID string  `json:"artifactId"`
	Scope      string  `json:"scope"`
	Files      []*item `json:"files"`
	FileCount  int     `json:"fileCount"`
	TotalSize  int64   `json:"totalSize"`
	ExpiresAt  int64   `json:"expiresAt"`
}

type registryEntry struct {
	Schema     string `json:"schema"`
	ArtifactID string `json:"artifactId"`
	URL        string `json:"url"`
	OwnerURL   string `json:"ownerUrl,omitempty"`
	Scope      string `json:"scope"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	OneTime    bool   `json:"oneTime"`
	Status     string `json:"status"`
}

type registryView struct {
	Schema     string `json:"schema"`
	ArtifactID string `json:"artifactId"`
	URL        string `json:"url"`
	Scope      string `json:"scope"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	OneTime    bool   `json:"oneTime"`
	Status     string `json:"status"`
}

func (e registryEntry) view() registryView {
	return registryView{Schema: e.Schema, ArtifactID: e.ArtifactID, URL: e.URL, Scope: e.Scope, CreatedAt: e.CreatedAt, ExpiresAt: e.ExpiresAt, OneTime: e.OneTime, Status: e.Status}
}

func cli() (bool, int) {
	if len(os.Args) < 2 {
		return false, 0
	}
	switch os.Args[1] {
	case "publish":
		return true, runPublish(os.Args[2:])
	case "inspect":
		return true, runInspect(os.Args[2:])
	case "receive":
		return true, runReceive(os.Args[2:])
	case "revoke":
		return true, runRevoke(os.Args[2:])
	case "list":
		return true, runList(os.Args[2:])
	case "status":
		return true, runStatus(os.Args[2:])
	case "serve":
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		return false, 0
	case "help", "--help", "-h":
		usage()
		return true, 0
	}
	return false, 0
}

func usage() {
	fmt.Println(`QQQShare
  qqqshare publish <files...> [--expires 5m] [--once] [--json]
  qqqshare inspect <url> [--json]
  qqqshare receive <url> [--output DIR] [--json]
  qqqshare list [--json]
  qqqshare status <artifact-id> [--json]
  qqqshare revoke <artifact-id|owner-url> [--json]
  qqqshare serve [desktop server options]`)
}

func runPublish(args []string) int {
	set := flag.NewFlagSet("publish", flag.ContinueOnError)
	expiry := set.Duration("expires", 5*time.Minute, "share duration")
	asJSON := set.Bool("json", false, "JSON output")
	maxMB := set.Int64("max-mb", 2048, "upload limit")
	oneTime := set.Bool("once", false, "expire each file after its first download attempt")
	if e := set.Parse(args); e != nil {
		return 2
	}
	paths := set.Args()
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "publish requires at least one file or folder")
		return 2
	}
	if *expiry < time.Second || *expiry > 24*time.Hour {
		fmt.Fprintln(os.Stderr, "expires must be 1s-24h")
		return 2
	}
	id := "art_" + randomToken(6)
	root := filepath.Join(dataRoot(""), "Artifacts", id)
	shared := filepath.Join(root, "Shared")
	if e := os.MkdirAll(shared, 0700); e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	for _, raw := range paths {
		abs, e := filepath.Abs(raw)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		st, e := os.Stat(abs)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		if st.IsDir() {
			dst := uniquePath(shared, cleanName(filepath.Base(abs))+".zip")
			if e = zipDir(abs, dst); e != nil {
				fmt.Fprintln(os.Stderr, e)
				return 1
			}
		} else {
			dst := uniquePath(shared, cleanName(filepath.Base(abs)))
			if e = copyFile(abs, dst); e != nil {
				fmt.Fprintln(os.Stderr, e)
				return 1
			}
		}
	}
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	token := randomToken(18)
	shareToken := randomToken(18)
	exe, _ := os.Executable()
	logFile, _ := os.OpenFile(filepath.Join(root, "server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	cmdArgs := []string{"serve", "--no-open", "--port", fmt.Sprint(port), "--token", token, "--share-token", shareToken, "--artifact-id", id, "--expires", expiry.String(), "--dir", root, "--max-mb", fmt.Sprint(*maxMB), "--exit-after", (*expiry + 30*time.Second).String()}
	if *oneTime {
		cmdArgs = append(cmdArgs, "--once")
	}
	cmd := exec.Command(exe, cmdArgs...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if e = cmd.Start(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	_ = cmd.Process.Release()
	_ = logFile.Close()
	client := http.Client{Timeout: time.Second}
	ready := false
	for n := 0; n < 30; n++ {
		r, e := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if e == nil {
			_ = r.Body.Close()
			if r.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		fmt.Fprintln(os.Stderr, "background server did not start")
		return 1
	}
	ip := "127.0.0.1"
	if list := localAddresses(); len(list) > 0 {
		ip = list[0]
	}
	shareURL := fmt.Sprintf("http://%s:%d/qqq?t=%s", ip, port, shareToken)
	ownerURL := fmt.Sprintf("http://127.0.0.1:%d/qqq?t=%s", port, token)
	now := time.Now()
	entry := registryEntry{Schema: "qqqshare-registry/v1", ArtifactID: id, URL: shareURL, OwnerURL: ownerURL, Scope: "lan", CreatedAt: now.UnixMilli(), ExpiresAt: now.Add(*expiry).UnixMilli(), OneTime: *oneTime, Status: "active"}
	if e = saveRegistryEntry(entry); e != nil {
		fmt.Fprintln(os.Stderr, "warning: registry:", e)
	}
	result := map[string]any{"schema": "qqqshare-publish/v1", "artifactId": id, "url": shareURL, "scope": "lan", "createdAt": entry.CreatedAt, "expiresAt": entry.ExpiresAt, "oneTime": *oneTime}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(result)
	} else {
		fmt.Printf("Published %s\nURL: %s\nExpires: %s\n", id, shareURL, time.Now().Add(*expiry).Format(time.RFC3339))
	}
	return 0
}

func runRevoke(args []string) int {
	set := flag.NewFlagSet("revoke", flag.ContinueOnError)
	asJSON := set.Bool("json", false, "JSON output")
	if set.Parse(args) != nil || set.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: qqqshare revoke <artifact-id|owner-url>")
		return 2
	}
	target := set.Arg(0)
	entry, found := loadRegistryEntry(target)
	if found {
		target = entry.OwnerURL
	}
	base, token, e := artifactEndpoint(target)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	req, _ := http.NewRequest(http.MethodPost, base+"/api/stop?t="+url.QueryEscape(token), nil)
	r, e := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, r.Status)
		return 1
	}
	if found {
		entry.Status = "revoked"
		_ = saveRegistryEntry(entry)
	}
	result := map[string]any{"schema": "qqqshare-revoke/v1", "artifactId": entry.ArtifactID, "revoked": true}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(result)
	} else {
		fmt.Println("Share revoked")
	}
	return 0
}

func registryDir() string { return filepath.Join(dataRoot(""), "Registry") }

func registryPath(id string) string { return filepath.Join(registryDir(), cleanName(id)+".json") }

func saveRegistryEntry(entry registryEntry) error {
	if e := os.MkdirAll(registryDir(), 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(entry, "", "  ")
	if e != nil {
		return e
	}
	tmp := registryPath(entry.ArtifactID) + ".tmp"
	if e = os.WriteFile(tmp, append(b, '\n'), 0600); e != nil {
		return e
	}
	target := registryPath(entry.ArtifactID)
	_ = os.Remove(target)
	return os.Rename(tmp, target)
}

func loadRegistryEntry(id string) (registryEntry, bool) {
	b, e := os.ReadFile(registryPath(id))
	if e != nil {
		return registryEntry{}, false
	}
	var entry registryEntry
	if json.Unmarshal(b, &entry) != nil || entry.ArtifactID == "" {
		return registryEntry{}, false
	}
	return entry, true
}

func registryEntries() []registryEntry {
	entries, _ := os.ReadDir(registryDir())
	out := []registryEntry{}
	for _, f := range entries {
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		b, e := os.ReadFile(filepath.Join(registryDir(), f.Name()))
		if e != nil {
			continue
		}
		var entry registryEntry
		if json.Unmarshal(b, &entry) != nil {
			continue
		}
		refreshEntryStatus(&entry)
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func refreshEntryStatus(entry *registryEntry) {
	if entry.Status != "active" {
		return
	}
	if time.Now().UnixMilli() >= entry.ExpiresAt {
		entry.Status = "expired"
		_ = saveRegistryEntry(*entry)
		return
	}
	u, e := url.Parse(entry.OwnerURL)
	if e != nil {
		return
	}
	r, e := (&http.Client{Timeout: 350 * time.Millisecond}).Get(u.Scheme + "://" + u.Host + "/health")
	if e != nil {
		entry.Status = "stopped"
		_ = saveRegistryEntry(*entry)
		return
	}
	_ = r.Body.Close()
	if r.StatusCode != http.StatusOK {
		entry.Status = "stopped"
		_ = saveRegistryEntry(*entry)
	}
}

func runList(args []string) int {
	set := flag.NewFlagSet("list", flag.ContinueOnError)
	asJSON := set.Bool("json", false, "JSON output")
	if set.Parse(args) != nil || set.NArg() != 0 {
		return 2
	}
	entries := registryEntries()
	views := make([]registryView, 0, len(entries))
	for _, entry := range entries {
		views = append(views, entry.view())
	}
	result := map[string]any{"schema": "qqqshare-list/v1", "artifacts": views, "count": len(views)}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(result)
		return 0
	}
	for _, e := range entries {
		fmt.Printf("%s  %-8s  expires %s  %s\n", e.ArtifactID, e.Status, time.UnixMilli(e.ExpiresAt).Format(time.RFC3339), e.URL)
	}
	return 0
}

func runStatus(args []string) int {
	set := flag.NewFlagSet("status", flag.ContinueOnError)
	asJSON := set.Bool("json", false, "JSON output")
	if set.Parse(args) != nil || set.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: qqqshare status <artifact-id>")
		return 2
	}
	entry, ok := loadRegistryEntry(set.Arg(0))
	if !ok {
		fmt.Fprintln(os.Stderr, "artifact not found")
		return 1
	}
	refreshEntryStatus(&entry)
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(entry.view())
	} else {
		fmt.Printf("%s: %s\nURL: %s\nExpires: %s\n", entry.ArtifactID, entry.Status, entry.URL, time.UnixMilli(entry.ExpiresAt).Format(time.RFC3339))
	}
	return 0
}

func artifactEndpoint(raw string) (string, string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return "", "", errors.New("invalid QQQShare URL")
	}
	token := u.Query().Get("t")
	if token == "" {
		return "", "", errors.New("URL has no access token")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || (!ip.IsPrivate() && !ip.IsLoopback()) {
		return "", "", errors.New("MVP only accepts private LAN or localhost IP addresses")
	}
	return u.Scheme + "://" + u.Host, token, nil
}
func fetchInfo(raw string) (artifactInfo, string, string, error) {
	base, token, e := artifactEndpoint(raw)
	if e != nil {
		return artifactInfo{}, "", "", e
	}
	c := http.Client{Timeout: 15 * time.Second}
	r, e := c.Get(base + "/api/artifact?t=" + url.QueryEscape(token))
	if e != nil {
		return artifactInfo{}, "", "", e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return artifactInfo{}, "", "", fmt.Errorf("server returned %s", r.Status)
	}
	var info artifactInfo
	e = json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&info)
	return info, base, token, e
}

func runInspect(args []string) int {
	set := flag.NewFlagSet("inspect", flag.ContinueOnError)
	asJSON := set.Bool("json", false, "JSON output")
	if set.Parse(args) != nil || set.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: qqqshare inspect <url>")
		return 2
	}
	info, _, _, e := fetchInfo(set.Arg(0))
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(info)
		return 0
	}
	fmt.Printf("Artifact: %s\nFiles: %d\nSize: %d bytes\nExpires: %s\n", info.Schema, info.FileCount, info.TotalSize, time.UnixMilli(info.ExpiresAt).Format(time.RFC3339))
	for _, f := range info.Files {
		fmt.Printf("- %s (%d bytes) sha256:%s\n", f.Name, f.Size, f.SHA256)
	}
	return 0
}

func runReceive(args []string) int {
	set := flag.NewFlagSet("receive", flag.ContinueOnError)
	out := set.String("output", "received", "output directory")
	asJSON := set.Bool("json", false, "JSON output")
	if set.Parse(args) != nil || set.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: qqqshare receive <url> [--output DIR]")
		return 2
	}
	info, base, token, e := fetchInfo(set.Arg(0))
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	if e = os.MkdirAll(*out, 0700); e != nil {
		fmt.Fprintln(os.Stderr, e)
		return 1
	}
	received := []string{}
	client := http.Client{Timeout: 30 * time.Minute}
	for _, f := range info.Files {
		dst := uniquePath(*out, cleanName(f.Name))
		r, e := client.Get(base + "/api/download/" + url.PathEscape(f.ID) + "?t=" + url.QueryEscape(token))
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		if r.StatusCode != 200 {
			_ = r.Body.Close()
			fmt.Fprintln(os.Stderr, r.Status)
			return 1
		}
		tmp := dst + ".part"
		w, e := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if e != nil {
			_ = r.Body.Close()
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		h := sha256.New()
		_, e = io.Copy(io.MultiWriter(w, h), io.LimitReader(r.Body, f.Size+1))
		_ = w.Close()
		_ = r.Body.Close()
		if e != nil || hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
			_ = os.Remove(tmp)
			fmt.Fprintln(os.Stderr, "hash verification failed:", f.Name)
			return 1
		}
		if e = os.Rename(tmp, dst); e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		received = append(received, dst)
	}
	result := map[string]any{"schema": "qqqshare-receive/v1", "verified": true, "files": received}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(result)
	} else {
		fmt.Printf("Received and verified %d file(s) in %s\n", len(received), *out)
	}
	return 0
}

func copyFile(src, dst string) error {
	r, e := os.Open(src)
	if e != nil {
		return e
	}
	defer r.Close()
	w, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(w, r)
	ce := w.Close()
	if e != nil {
		return e
	}
	return ce
}
func zipDir(root, dst string) error {
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	z := zip.NewWriter(out)
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(filepath.Dir(root), path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		h, e := zip.FileInfoHeader(mustInfo(d))
		if e != nil {
			return e
		}
		h.Name = rel
		h.Method = zip.Deflate
		w, e := z.CreateHeader(h)
		if e != nil {
			return e
		}
		r, e := os.Open(path)
		if e != nil {
			return e
		}
		_, copyErr := io.Copy(w, r)
		_ = r.Close()
		return copyErr
	})
	closeErr := z.Close()
	fileErr := out.Close()
	if walkErr != nil {
		return walkErr
	}
	if closeErr != nil {
		return closeErr
	}
	return fileErr
}
func mustInfo(d os.DirEntry) os.FileInfo {
	i, e := d.Info()
	if e != nil {
		panic(e)
	}
	return i
}
