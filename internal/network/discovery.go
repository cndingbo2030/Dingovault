package network

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

// DingovaultService is the mDNS DNS-SD service name for peer discovery.
const DingovaultService = "_dingovault._tcp"

// Peer is one Dingovault instance discovered on the local network.
type Peer struct {
	Name string   `json:"name"`
	Host string   `json:"host"`
	IP   string   `json:"ip"`
	Port int      `json:"port"`
	TXT  []string `json:"txt,omitempty"`
}

// BrowseDingovaultPeers runs a short mDNS query for other Dingovault advertisers on the LAN.
func BrowseDingovaultPeers(ctx context.Context, timeout time.Duration) ([]Peer, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	entries := make(chan *mdns.ServiceEntry, 32)
	params := &mdns.QueryParam{
		Service:             DingovaultService,
		Domain:              "local",
		Timeout:             timeout,
		Entries:             entries,
		WantUnicastResponse: true,
	}
	go func() { _ = mdns.QueryContext(ctx, params) }()

	merge := func(e *mdns.ServiceEntry, seen map[string]Peer) {
		if e == nil {
			return
		}
		ip := ""
		if e.AddrV4 != nil {
			ip = e.AddrV4.String()
		} else if e.Addr != nil {
			ip = e.Addr.String()
		}
		if ip == "" || e.Port <= 0 {
			return
		}
		key := ip + ":" + fmt.Sprint(e.Port)
		seen[key] = Peer{
			Name: strings.TrimSuffix(e.Name, "."),
			Host: e.Host,
			IP:   ip,
			Port: e.Port,
			TXT:  append([]string(nil), e.InfoFields...),
		}
	}

	seen := make(map[string]Peer)
	deadline := time.NewTimer(timeout + 400*time.Millisecond)
	defer deadline.Stop()
	for {
		select {
		case e := <-entries:
			merge(e, seen)
		case <-deadline.C:
			out := make([]Peer, 0, len(seen))
			for _, p := range seen {
				out = append(out, p)
			}
			return out, nil
		case <-ctx.Done():
			out := make([]Peer, 0, len(seen))
			for _, p := range seen {
				out = append(out, p)
			}
			return out, ctx.Err()
		}
	}
}
