// Package mitv talks to the LAN remote API of Xiaomi HyperOS displays and
// TVs: the HTTP server on port 6095 that switches inputs and changes volume.
package mitv

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Port is where the remote API listens.
const Port = 6095

// maxResponseSize caps HTTP response bodies to prevent OOM from untrusted network endpoints.
const maxResponseSize = 64 * 1024

// ErrRejected means the device answered but refused the request.
var ErrRejected = errors.New("mitv: request rejected")

// Source is an input of the display, such as HDMI1 (23) or DP (29).
type Source struct {
	ID   int
	Name string
}

// lanSafeProxy bypasses environment HTTP proxies for direct LAN/loopback IPs,
// mDNS .local names, and single-label local hostnames, while honoring
// http.ProxyFromEnvironment for any routed domain.
func lanSafeProxy(req *http.Request) (*url.URL, error) {
	host := req.URL.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return nil, nil
		}
	}
	lower := strings.ToLower(host)
	if strings.HasSuffix(lower, ".local") || (lower != "" && !strings.Contains(lower, ".")) {
		return nil, nil
	}
	return http.ProxyFromEnvironment(req)
}

// sharedClient pools persistent TCP connections across LAN calls and discovery probes.
var sharedClient = &http.Client{
	Timeout: 4 * time.Second,
	Transport: &http.Transport{
		Proxy:               lanSafeProxy,
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	},
}

// Link is a client for one device.
type Link struct {
	Host   string
	client *http.Client
}

func cleanHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" || strings.ContainsAny(host, " :/?#@[]\\\r\n\t") {
		return ""
	}
	return host
}

func New(host string) *Link {
	return &Link{Host: cleanHost(host), client: sharedClient}
}

// DeviceName answers whether the device is reachable, with its name.
func (l *Link) DeviceName() (string, error) {
	return l.DeviceNameContext(context.Background())
}

// DeviceNameTimeout answers whether the device is reachable within d.
func (l *Link) DeviceNameTimeout(d time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return l.DeviceNameContext(ctx)
}

// DeviceNameContext answers whether the device is reachable, with context cancellation support.
func (l *Link) DeviceNameContext(ctx context.Context) (string, error) {
	var data struct {
		DeviceName string `json:"devicename"`
	}
	err := l.callContext(ctx, "request", url.Values{"action": {"isalive"}}, &data)
	return data.DeviceName, err
}

func (l *Link) Sources() ([]Source, error) {
	var data struct {
		SourceList []struct {
			SourceID   int    `json:"sourceId"`
			SourceName string `json:"sourceName"`
		} `json:"sourceList"`
	}
	if err := l.call("controller", url.Values{"action": {"getsources"}}, &data); err != nil {
		return nil, err
	}
	sources := make([]Source, len(data.SourceList))
	for i, s := range data.SourceList {
		sources[i] = Source{ID: s.SourceID, Name: s.SourceName}
	}
	return sources, nil
}

// Select switches input. changesource2 takes the numeric id, so it also
// reaches DP and USB-C, which the name-based changesource rejects.
func (l *Link) Select(sourceID int) error {
	return l.call("controller", url.Values{"action": {"changesource2"}, "sourceid": {strconv.Itoa(sourceID)}}, nil)
}

// VolumeRaw reads the device's raw volume step and maximum volume step.
func (l *Link) VolumeRaw() (volume, maxVolume int, err error) {
	var data struct {
		Volume    int `json:"volume"`
		MaxVolume int `json:"maxVolume"`
	}
	if err := l.call("controller", url.Values{"action": {"getvolume"}}, &data); err != nil {
		return 0, 0, err
	}
	if data.MaxVolume <= 0 || data.MaxVolume > 1000 || data.Volume < 0 {
		return 0, 0, errors.New("mitv: invalid volume range")
	}
	vol := min(data.Volume, data.MaxVolume)
	return vol, data.MaxVolume, nil
}

// Volume reads the volume on a 0...100 scale, rounded to the nearest integer.
func (l *Link) Volume() (int, error) {
	vol, maxVol, err := l.VolumeRaw()
	if err != nil {
		return 0, err
	}
	pct := (vol*100 + maxVol/2) / maxVol
	return max(0, min(pct, 100)), nil
}

// Press sends a remote key such as "volumeup", "volumedown", "home" or "back".
func (l *Link) Press(key string) error {
	return l.call("controller", url.Values{"action": {"keyevent"}, "keycode": {key}}, nil)
}

func (l *Link) call(path string, query url.Values, data any) error {
	return l.callContext(context.Background(), path, query, data)
}

func (l *Link) callContext(ctx context.Context, path string, query url.Values, data any) error {
	host := cleanHost(l.Host)
	if host == "" {
		return errors.New("mitv: invalid host")
	}
	u := url.URL{
		Scheme:   "http",
		Host:     net.JoinHostPort(host, strconv.Itoa(Port)),
		Path:     "/" + path,
		RawQuery: query.Encode(),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var envelope struct {
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	lr := io.LimitReader(resp.Body, maxResponseSize)
	if err := json.NewDecoder(lr).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Msg != "success" {
		return ErrRejected
	}
	if data == nil || len(envelope.Data) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Data, data)
}
