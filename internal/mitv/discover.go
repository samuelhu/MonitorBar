package mitv

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

// ErrNotFound means no HyperOS display or TV answered on the local network.
var ErrNotFound = errors.New("mitv: no device found")

func Discover() (string, error) {
	var (
		mu      sync.Mutex
		display string
		other   string
		wg      sync.WaitGroup
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Throttle concurrent dials to prevent socket exhaustion while scanning /24 promptly.
	sem := make(chan struct{}, 64)

	dialer := net.Dialer{Timeout: 500 * time.Millisecond}

	for _, host := range candidates() {
		select {
		case <-ctx.Done():
			break
		default:
		}

		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(target, "6095"))
			if err != nil {
				return
			}
			conn.Close()

			name, err := New(target).DeviceNameContext(ctx)
			if err != nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()
			if strings.Contains(name, "显示器") || strings.Contains(strings.ToLower(name), "monitor") {
				display = target
				cancel() // Early exit when the preferred display is found
			} else if other == "" {
				other = target
			}
		}(host)
	}
	wg.Wait()
	switch {
	case display != "":
		return display, nil
	case other != "":
		return other, nil
	}
	return "", ErrNotFound
}

// candidates lists the other addresses of each private IPv4 /24 we are on.
func candidates() []string {
	var hosts []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil || ip.IsLoopback() || !ip.IsPrivate() {
			continue
		}
		for last := 1; last < 255; last++ {
			if byte(last) != ip[3] {
				hosts = append(hosts, net.IPv4(ip[0], ip[1], ip[2], byte(last)).String())
			}
		}
	}
	return hosts
}
