package update

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testModule = "github.com/queone/govna"

var start = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

// proxy serves @latest replies and counts requests.
type proxy struct {
	server   *httptest.Server
	requests atomic.Int32
	lastPath atomic.Value
}

func newProxy(t *testing.T, handler func(w http.ResponseWriter)) *proxy {
	t.Helper()
	p := &proxy{}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.requests.Add(1)
		p.lastPath.Store(r.URL.Path)
		handler(w)
	}))
	t.Cleanup(p.server.Close)
	return p
}

func latest(version string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) { w.Write([]byte(`{"Version":"` + version + `"}`)) }
}

func noEnv(string) (string, bool) { return "", false }

func envOf(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func options(t *testing.T, p *proxy, now *time.Time) Options {
	t.Helper()
	return Options{
		Module:    testModule,
		Current:   "v0.46.0",
		CacheFile: filepath.Join(t.TempDir(), "cache", "govna", "update-check"),
		Proxy:     p.server.URL,
		Now:       func() time.Time { return *now },
		LookupEnv: noEnv,
	}
}

func TestCheckReportsLatestRelease(t *testing.T) {
	cases := []struct {
		name, reply, latest string
		available           bool
	}{
		{"newer release", "v0.47.0", "v0.47.0", true},
		{"current release", "v0.46.0", "v0.46.0", false},
		{"older release", "v0.45.5", "v0.45.5", false},
		{"pre-release ignored", "v0.47.0-rc.1", "", false},
		{"pseudo-version ignored", "v0.0.0-20261029120000-abcdef123456", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newProxy(t, latest(tc.reply))
			now := start
			r, err := Check(options(t, p, &now))
			if err != nil {
				t.Fatal(err)
			}
			if r.Latest != tc.latest || r.Available != tc.available || r.FromCache || r.Skipped != "" || !r.CheckedAt.Equal(start) {
				t.Fatalf("result=%+v, want latest %q available %v", r, tc.latest, tc.available)
			}
		})
	}
}

func TestNewerComparesStrictReleases(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.47.0", "v0.46.0", true},
		{"v0.46.10", "v0.46.9", true},
		{"v1.0.0", "v0.99.99", true},
		{"v0.46.0", "v0.46.0", false},
		{"v0.45.0", "v0.46.0", false},
		{"v0.47.0-rc.1", "v0.46.0", false},
		{"0.47.0", "v0.46.0", false},
		{"v0.47.0", "dev", false},
		{"v01.0.0", "v0.1.0", false},
	}
	for _, tc := range cases {
		if got := Newer(tc.a, tc.b); got != tc.want {
			t.Errorf("Newer(%q, %q)=%v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCheckEscapesUppercaseModulePath(t *testing.T) {
	p := newProxy(t, latest("v1.0.0"))
	now := start
	o := options(t, p, &now)
	o.Module = "github.com/Queone/GovNa"
	if _, err := Check(o); err != nil {
		t.Fatal(err)
	}
	if got := p.lastPath.Load(); got != "/github.com/!queone/!gov!na/@latest" {
		t.Fatalf("request path=%v", got)
	}
}

func TestCheckRejectsOversizedReply(t *testing.T) {
	p := newProxy(t, func(w http.ResponseWriter) {
		w.Write([]byte(`{"Version":"v0.47.0","Pad":"` + strings.Repeat("x", maxBody) + `"}`))
	})
	now := start
	if _, err := Check(options(t, p, &now)); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err=%v, want oversized reply error", err)
	}
}

func TestCheckFailsWithinTimeout(t *testing.T) {
	release := make(chan struct{})
	p := newProxy(t, func(http.ResponseWriter) { <-release })
	t.Cleanup(func() { close(release) })
	now := start
	o := options(t, p, &now)
	o.Timeout = 50 * time.Millisecond
	began := time.Now()
	_, err := Check(o)
	if err == nil {
		t.Fatal("slow proxy did not fail")
	}
	if elapsed := time.Since(began); elapsed > time.Second {
		t.Fatalf("check took %v, want the 50ms timeout", elapsed)
	}
}

func TestCheckReusesCacheWithinInterval(t *testing.T) {
	p := newProxy(t, latest("v0.47.0"))
	now := start
	o := options(t, p, &now)
	if _, err := Check(o); err != nil {
		t.Fatal(err)
	}
	now = start.Add(23 * time.Hour)
	r, err := Check(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.requests.Load() != 1 || !r.FromCache || r.Latest != "v0.47.0" || !r.Available || !r.CheckedAt.Equal(start) {
		t.Fatalf("requests=%d result=%+v, want one cached answer", p.requests.Load(), r)
	}
	now = start.Add(25 * time.Hour)
	r, err = Check(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.requests.Load() != 2 || r.FromCache || !r.CheckedAt.Equal(now) {
		t.Fatalf("requests=%d result=%+v, want a fresh request after the interval", p.requests.Load(), r)
	}
}

func TestCheckWaitsAnHourAfterFailure(t *testing.T) {
	fail := atomic.Bool{}
	fail.Store(true)
	p := newProxy(t, func(w http.ResponseWriter) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		latest("v0.47.0")(w)
	})
	now := start
	o := options(t, p, &now)
	if _, err := Check(o); err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("err=%v, want proxy failure", err)
	}
	fail.Store(false)
	now = start.Add(59 * time.Minute)
	r, err := Check(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.requests.Load() != 1 || r.Skipped != SkippedRecentFailure || r.Latest != "" {
		t.Fatalf("requests=%d result=%+v, want a skipped retry", p.requests.Load(), r)
	}
	now = start.Add(61 * time.Minute)
	r, err = Check(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.requests.Load() != 2 || r.Latest != "v0.47.0" || r.Skipped != "" {
		t.Fatalf("requests=%d result=%+v, want a retry after one hour", p.requests.Load(), r)
	}
}

func TestCheckWritesPrivateCache(t *testing.T) {
	p := newProxy(t, latest("v0.47.0"))
	now := start
	o := options(t, p, &now)
	if _, err := Check(o); err != nil {
		t.Fatal(err)
	}
	file, err := os.Stat(o.CacheFile)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.Stat(filepath.Dir(o.CacheFile))
	if err != nil {
		t.Fatal(err)
	}
	if file.Mode().Perm() != 0o600 || dir.Mode().Perm() != 0o700 {
		t.Fatalf("file mode=%v dir mode=%v, want 0600 and 0700", file.Mode().Perm(), dir.Mode().Perm())
	}
}

func TestCheckSkipsWithoutRequest(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		current string
		proxy   bool
		reason  string
	}{
		{"NO_UPDATE_CHECK set", map[string]string{"NO_UPDATE_CHECK": "1"}, "v0.46.0", true, "NO_UPDATE_CHECK is set"},
		{"GOPROXY off", map[string]string{"GOPROXY": "off"}, "v0.46.0", false, `GOPROXY entry "off" is not an http or https proxy`},
		{"GOPROXY direct", map[string]string{"GOPROXY": "direct"}, "v0.46.0", false, `GOPROXY entry "direct" is not an http or https proxy`},
		{"GOPROXY file URL", map[string]string{"GOPROXY": "file:///tmp/mods,https://proxy.golang.org"}, "v0.46.0", false, `GOPROXY entry "file:///tmp/mods" is not an http or https proxy`},
		{"development build", nil, "dev", true, `version "dev" is not a release`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newProxy(t, latest("v0.47.0"))
			now := start
			o := options(t, p, &now)
			o.Current = tc.current
			o.LookupEnv = envOf(tc.env)
			if !tc.proxy {
				o.Proxy = ""
			}
			r, err := Check(o)
			if err != nil {
				t.Fatal(err)
			}
			if r.Skipped != tc.reason || p.requests.Load() != 0 {
				t.Fatalf("skipped=%q requests=%d, want %q and none", r.Skipped, p.requests.Load(), tc.reason)
			}
			if _, err := os.Stat(o.CacheFile); !os.IsNotExist(err) {
				t.Fatalf("skipped check touched the cache: %v", err)
			}
		})
	}
}

func TestCheckUsesFirstGOPROXYEntry(t *testing.T) {
	p := newProxy(t, latest("v0.47.0"))
	now := start
	o := options(t, p, &now)
	o.Proxy = ""
	o.LookupEnv = envOf(map[string]string{"GOPROXY": p.server.URL + "/,direct"})
	r, err := Check(o)
	if err != nil {
		t.Fatal(err)
	}
	if p.requests.Load() != 1 || r.Latest != "v0.47.0" {
		t.Fatalf("requests=%d result=%+v, want the first GOPROXY entry", p.requests.Load(), r)
	}
}
