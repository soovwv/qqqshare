package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

//go:embed src/web.js assets/favicon.png
var embedded embed.FS

type item struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
	SHA256    string `json:"sha256"`
	Downloads int64  `json:"downloads"`
	path      string
	active    bool
}

type app struct {
	mu                                            sync.RWMutex
	root, shared, expired, ownerToken, shareToken string
	duration                                      time.Duration
	maxUpload                                     int64
	files                                         map[string]*item
	port                                          int
	stop                                          context.CancelFunc
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func dataRoot(override string) string {
	if override != "" {
		p, _ := filepath.Abs(override)
		return p
	}
	if runtime.GOOS == "darwin" {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, "Library", "Application Support", "QQQShare")
	}
	if p := os.Getenv("LOCALAPPDATA"); p != "" {
		return filepath.Join(p, "QQQShare")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".qqqshare")
}

func newApp(root, ownerToken, shareToken string, duration time.Duration, maxMB int64) (*app, error) {
	a := &app{root: root, shared: filepath.Join(root, "Shared"), expired: filepath.Join(root, "Expired"), ownerToken: ownerToken, shareToken: shareToken, duration: duration, maxUpload: maxMB << 20, files: map[string]*item{}}
	for _, d := range []string{a.shared, a.expired} {
		if e := os.MkdirAll(d, 0700); e != nil {
			return nil, e
		}
	}
	return a, nil
}

func (a *app) register(path, name string) (*item, error) {
	st, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		_ = f.Close()
		return nil, e
	}
	_ = f.Close()
	now := time.Now()
	i := &item{ID: randomToken(12), Name: name, Size: st.Size(), CreatedAt: now.UnixMilli(), ExpiresAt: now.Add(a.duration).UnixMilli(), SHA256: hex.EncodeToString(h.Sum(nil)), path: path, active: true}
	a.mu.Lock()
	a.files[path] = i
	a.mu.Unlock()
	return i, nil
}

func (a *app) discover() {
	entries, e := os.ReadDir(a.shared)
	if e != nil {
		return
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || strings.Contains(entry.Name(), ".uploading-") {
			continue
		}
		p := filepath.Join(a.shared, entry.Name())
		seen[p] = true
		a.mu.RLock()
		_, ok := a.files[p]
		a.mu.RUnlock()
		if !ok {
			_, _ = a.register(p, entry.Name())
		}
	}
	a.mu.Lock()
	for p, i := range a.files {
		if i.active && !seen[p] {
			delete(a.files, p)
		}
	}
	a.mu.Unlock()
}

func cleanName(raw string) string {
	n := filepath.Base(strings.TrimSpace(raw))
	if n == "." || n == "" {
		return "upload"
	}
	return strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, n)
}

func uniquePath(dir, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for n := 1; ; n++ {
		candidate := filepath.Join(dir, name)
		if n > 1 {
			candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, n, ext))
		}
		if _, e := os.Stat(candidate); errors.Is(e, fs.ErrNotExist) {
			return candidate
		}
	}
}

func (a *app) expire() {
	now := time.Now().UnixMilli()
	var due []*item
	a.mu.Lock()
	for _, i := range a.files {
		if i.active && i.ExpiresAt <= now {
			i.active = false
			due = append(due, i)
		}
	}
	a.mu.Unlock()
	for _, i := range due {
		dst := uniquePath(a.expired, i.Name)
		if e := os.Rename(i.path, dst); e != nil {
			log.Printf("archive %q: %v", i.Name, e)
		} else {
			i.path = dst
		}
	}
}

func (a *app) list() []*item {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := []*item{}
	for _, i := range a.files {
		if i.active {
			copy := *i
			copy.path = ""
			out = append(out, &copy)
		}
	}
	sort.Slice(out, func(x, y int) bool { return out[x].CreatedAt > out[y].CreatedAt })
	return out
}

func (a *app) find(id string) *item {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, i := range a.files {
		if i.active && i.ID == id {
			return i
		}
	}
	return nil
}

func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *app) role(r *http.Request) string {
	t := r.URL.Query().Get("t")
	if t != "" && t == a.ownerToken {
		return "owner"
	}
	if t != "" && t == a.shareToken {
		return "reader"
	}
	return ""
}

func page() ([]byte, error) {
	b, e := embedded.ReadFile("src/web.js")
	if e != nil {
		return nil, e
	}
	s := string(b)
	const p = "export const page = `"
	if !strings.HasPrefix(s, p) {
		return nil, errors.New("invalid embedded web page")
	}
	s = strings.TrimPrefix(s, p)
	s = strings.TrimSuffix(strings.TrimSpace(s), "`;")
	return []byte(s), nil
}

func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	html, _ := page()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/qqq?"+r.URL.RawQuery, http.StatusFound)
	})
	mux.HandleFunc("GET /qqq", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(html)
	})
	mux.HandleFunc("GET /favicon.png", func(w http.ResponseWriter, r *http.Request) {
		b, _ := embedded.ReadFile("assets/favicon.png")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, 200, map[string]any{"ok": true, "version": version})
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		role := a.role(r)
		if role == "" {
			jsonOut(w, 403, map[string]string{"error": "Invalid share token"})
			return
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/info":
			urls := []string{}
			for _, ip := range localAddresses() {
				urls = append(urls, fmt.Sprintf("http://%s:%d/qqq?t=%s", ip, a.port, a.shareToken))
			}
			jsonOut(w, 200, map[string]any{"urls": urls, "role": role, "scope": "lan", "version": version})
		case r.Method == "GET" && r.URL.Path == "/api/files":
			a.mu.RLock()
			d := int(a.duration.Seconds())
			a.mu.RUnlock()
			jsonOut(w, 200, map[string]any{"duration": d, "files": a.list()})
		case r.Method == "GET" && r.URL.Path == "/api/qr":
			ip := "127.0.0.1"
			if xs := localAddresses(); len(xs) > 0 {
				ip = xs[0]
			}
			png, e := qrcode.Encode(fmt.Sprintf("http://%s:%d/qqq?t=%s", ip, a.port, a.shareToken), qrcode.Medium, 256)
			if e != nil {
				jsonOut(w, 500, map[string]string{"error": e.Error()})
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(png)
		case r.Method == "GET" && r.URL.Path == "/api/artifact":
			files := a.list()
			var total int64
			var expires int64
			for _, i := range files {
				total += i.Size
				if expires == 0 || i.ExpiresAt < expires {
					expires = i.ExpiresAt
				}
			}
			jsonOut(w, 200, map[string]any{"schema": "qqqshare-artifact/v1", "artifactId": "live", "scope": "lan", "files": files, "fileCount": len(files), "totalSize": total, "expiresAt": expires})
		case r.Method == "PUT" && r.URL.Path == "/api/settings":
			if role != "owner" {
				jsonOut(w, 403, map[string]string{"error": "Owner token required"})
				return
			}
			var body struct {
				Duration int `json:"duration"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body) != nil || body.Duration < 1 || body.Duration > 86400 {
				jsonOut(w, 400, map[string]string{"error": "Duration must be 1-86400 seconds"})
				return
			}
			a.mu.Lock()
			a.duration = time.Duration(body.Duration) * time.Second
			a.mu.Unlock()
			jsonOut(w, 200, body)
		case r.Method == "POST" && r.URL.Path == "/api/upload":
			if role != "owner" {
				jsonOut(w, 403, map[string]string{"error": "Owner token required"})
				return
			}
			a.upload(w, r)
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/files/"):
			if role != "owner" {
				jsonOut(w, 403, map[string]string{"error": "Owner token required"})
				return
			}
			if !a.revoke(strings.TrimPrefix(r.URL.Path, "/api/files/")) {
				jsonOut(w, 404, map[string]string{"error": "File expired or missing"})
				return
			}
			jsonOut(w, 200, map[string]bool{"revoked": true})
		case r.Method == "POST" && r.URL.Path == "/api/stop":
			if role != "owner" {
				jsonOut(w, 403, map[string]string{"error": "Owner token required"})
				return
			}
			jsonOut(w, 200, map[string]bool{"stopping": true})
			if a.stop != nil {
				go a.stop()
			}
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/download/"):
			a.download(w, r)
		default:
			jsonOut(w, 404, map[string]string{"error": "Not found"})
		}
	})
	return securityHeaders(mux)
}

func (a *app) upload(w http.ResponseWriter, r *http.Request) {
	name := cleanName(r.URL.Query().Get("name"))
	dst := uniquePath(a.shared, name)
	tmp := dst + ".uploading-" + randomToken(6)
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		jsonOut(w, 500, map[string]string{"error": e.Error()})
		return
	}
	reader := http.MaxBytesReader(w, r.Body, a.maxUpload)
	_, e = io.Copy(f, reader)
	closeErr := f.Close()
	if e != nil || closeErr != nil {
		_ = os.Remove(tmp)
		jsonOut(w, 413, map[string]string{"error": "Upload failed or exceeded limit"})
		return
	}
	if e = os.Rename(tmp, dst); e != nil {
		_ = os.Remove(tmp)
		jsonOut(w, 500, map[string]string{"error": e.Error()})
		return
	}
	i, e := a.register(dst, filepath.Base(dst))
	if e != nil {
		jsonOut(w, 500, map[string]string{"error": e.Error()})
		return
	}
	jsonOut(w, 201, map[string]any{"id": i.ID, "expiresAt": i.ExpiresAt})
}

func (a *app) download(w http.ResponseWriter, r *http.Request) {
	i := a.find(strings.TrimPrefix(r.URL.Path, "/api/download/"))
	if i == nil {
		jsonOut(w, 404, map[string]string{"error": "File expired or missing"})
		return
	}
	f, e := os.Open(i.path)
	if e != nil {
		jsonOut(w, 404, map[string]string{"error": "File missing"})
		return
	}
	defer f.Close()
	a.mu.Lock()
	i.Downloads++
	a.mu.Unlock()
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(i.Name)))
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": i.Name}))
	w.Header().Set("Content-Length", strconv.FormatInt(i.Size, 10))
	_, _ = io.Copy(w, f)
}

func (a *app) revoke(id string) bool {
	a.mu.Lock()
	var target *item
	for _, i := range a.files {
		if i.active && i.ID == id {
			i.active = false
			target = i
			break
		}
	}
	a.mu.Unlock()
	if target == nil {
		return false
	}
	dst := uniquePath(a.expired, target.Name)
	if e := os.Rename(target.path, dst); e != nil {
		log.Printf("revoke %q: %v", target.Name, e)
	} else {
		target.path = dst
	}
	return true
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func localAddresses() []string {
	ifaces, _ := net.Interfaces()
	type candidate struct {
		ip, name string
		score    int
	}
	var list []candidate
	for _, in := range ifaces {
		if in.Flags&net.FlagUp == 0 || in.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := in.Addrs()
		for _, addr := range addrs {
			ip, _, _ := net.ParseCIDR(addr.String())
			if ip == nil || ip.To4() == nil {
				continue
			}
			s := 0
			n := strings.ToLower(in.Name)
			if strings.Contains(n, "vethernet") || strings.Contains(n, "wsl") || strings.Contains(n, "docker") || strings.Contains(n, "vmware") || strings.Contains(n, "virtualbox") || strings.Contains(n, "tailscale") {
				s -= 100
			}
			v := ip.To4()
			if v[0] == 192 && v[1] == 168 {
				s += 40
			} else if v[0] == 10 {
				s += 30
			} else if v[0] == 172 && v[1] >= 16 && v[1] <= 31 {
				s += 20
			}
			if strings.Contains(n, "wi-fi") || strings.Contains(n, "wireless") || strings.Contains(n, "ethernet") {
				s += 10
			}
			list = append(list, candidate{ip.String(), in.Name, s})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].score > list[j].score })
	out := []string{}
	for _, x := range list {
		out = append(out, x.ip)
	}
	return out
}

func openBrowser(url string) {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	} else if runtime.GOOS == "darwin" {
		c = exec.Command("open", url)
	} else {
		c = exec.Command("xdg-open", url)
	}
	_ = c.Start()
}

var version = "dev"

func main() {
	if handled, code := cli(); handled {
		os.Exit(code)
	}
	port := flag.Int("port", 0, "fixed port (default: random)")
	expiry := flag.Duration("expires", time.Minute, "default share duration")
	dir := flag.String("dir", "", "data directory")
	maxMB := flag.Int64("max-mb", 2048, "maximum upload size in MiB")
	noOpen := flag.Bool("no-open", false, "do not open the browser")
	exitAfter := flag.Duration("exit-after", 0, "stop server automatically")
	token := flag.String("token", "", "fixed access token")
	shareToken := flag.String("share-token", "", "fixed read-only share token")
	flag.Parse()
	if *expiry < time.Second || *expiry > 24*time.Hour || *maxMB < 1 {
		log.Fatal("invalid expiration or upload limit")
	}
	if *token == "" {
		*token = randomToken(18)
	}
	if *shareToken == "" {
		*shareToken = randomToken(18)
	}
	a, e := newApp(dataRoot(*dir), *token, *shareToken, *expiry, *maxMB)
	if e != nil {
		log.Fatal(e)
	}
	ln, e := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if e != nil {
		log.Fatalf("cannot start: %v", e)
	}
	a.port = ln.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: a.handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 1 << 20}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	a.stop = cancel
	a.discover()
	if *exitAfter > 0 {
		time.AfterFunc(*exitAfter, cancel)
	}
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		discover := time.NewTicker(time.Second)
		defer discover.Stop()
		for {
			select {
			case <-ticker.C:
				a.expire()
			case <-discover.C:
				a.discover()
			case <-ctx.Done():
				return
			}
		}
	}()
	url := fmt.Sprintf("http://localhost:%d/qqq?t=%s", a.port, a.ownerToken)
	fmt.Printf("QQQShare %s\nLocal: %s\n", version, url)
	for _, ip := range localAddresses() {
		fmt.Printf("LAN:   http://%s:%d/qqq?t=%s\n", ip, a.port, a.shareToken)
	}
	fmt.Printf("Shared: %s\n", a.shared)
	if !*noOpen {
		openBrowser(url)
	}
	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = server.Shutdown(shutdown)
	}()
	if e = server.Serve(ln); e != nil && !errors.Is(e, http.ErrServerClosed) {
		log.Fatal(e)
	}
}
