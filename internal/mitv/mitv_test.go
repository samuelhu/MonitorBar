package mitv

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestVolumeRejectsInvalidRangeAndClamps(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	link := New("127.0.0.1")
	link.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Host = u.Host
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	// Finding #1: maxVolume=0 with volume=50 must return an error, never 5000.
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{"zero maxVolume", `{"msg":"success","data":{"volume":50,"maxVolume":0}}`},
		{"missing maxVolume", `{"msg":"success","data":{"volume":50}}`},
		{"negative maxVolume", `{"msg":"success","data":{"volume":10,"maxVolume":-5}}`},
		{"negative volume", `{"msg":"success","data":{"volume":-1,"maxVolume":100}}`},
		{"excessive maxVolume", `{"msg":"success","data":{"volume":10,"maxVolume":5000}}`},
	} {
		body = tc.payload
		if v, err := link.Volume(); err == nil {
			t.Errorf("%s: expected error, got volume %d", tc.name, v)
		}
		if raw, maxV, err := link.VolumeRaw(); err == nil {
			t.Errorf("%s: expected VolumeRaw error, got %d/%d", tc.name, raw, maxV)
		}
	}

	// Out-of-ceiling volume > maxVolume is clamped to maxVolume (100%).
	body = `{"msg":"success","data":{"volume":99,"maxVolume":15}}`
	raw, maxV, err := link.VolumeRaw()
	if err != nil || raw != 15 || maxV != 15 {
		t.Fatalf("expected clamped 15/15, got %d/%d (err=%v)", raw, maxV, err)
	}
	pct, err := link.Volume()
	if err != nil || pct != 100 {
		t.Fatalf("expected 100%%, got %d%% (err=%v)", pct, err)
	}

	// Normal 15-step device: 6/15 -> 40%.
	body = `{"msg":"success","data":{"volume":6,"maxVolume":15}}`
	raw, maxV, err = link.VolumeRaw()
	if err != nil || raw != 6 || maxV != 15 {
		t.Fatalf("expected 6/15, got %d/%d (err=%v)", raw, maxV, err)
	}
	pct, err = link.Volume()
	if err != nil || pct != 40 {
		t.Fatalf("expected 40%%, got %d%% (err=%v)", pct, err)
	}
}

func TestHostValidationRejectsPortAndPathInjection(t *testing.T) {
	for _, bad := range []string{
		"",
		"   ",
		"192.168.1.10:8080",
		"::1",
		"[::1]",
		"192.168.1.10/path",
		"192.168.1.10?x=1",
		"user@192.168.1.10",
		"192.168.1.10#frag",
	} {
		l := New(bad)
		if l.Host != "" {
			t.Errorf("New(%q).Host = %q, want empty", bad, l.Host)
		}
		if _, err := l.DeviceName(); err == nil {
			t.Errorf("New(%q).DeviceName() succeeded, want error", bad)
		}
		// Direct struct mutation must also be rejected by callContext.
		raw := &Link{Host: bad, client: sharedClient}
		if _, err := raw.DeviceName(); err == nil {
			t.Errorf("Link{Host: %q}.DeviceName() succeeded, want error", bad)
		}
	}
}

func TestSharedClientPreservesProxyFromEnvironment(t *testing.T) {
	tr, ok := sharedClient.Transport.(*http.Transport)
	if !ok || tr.Proxy == nil {
		t.Fatalf("expected sharedClient.Transport to preserve http.ProxyFromEnvironment")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
