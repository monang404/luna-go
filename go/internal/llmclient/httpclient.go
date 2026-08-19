// This file provides the shared *http.Client used by CallBlocking and
// CallStreaming. It exists to work around a Termux/Android-specific bug:
// Go's default DNS resolver reads /etc/resolv.conf to find a nameserver,
// but Android does not populate that file (DNS is instead handled
// internally by netd, outside any file the Go runtime can see). With no
// nameserver configured, Go's resolver falls back to querying
// localhost:53, which has nothing listening on it, and every request
// fails with "dial tcp: lookup <host> on [::1]:53: connect: connection
// refused" -- regardless of which provider or API key is in play.
//
// curl works in the same shell because it goes through libc's resolver,
// which Android *does* wire up correctly. Go's pure-Go resolver has no
// such wiring. The fix here is to give the HTTP client's dialer a
// net.Resolver whose Dial func talks to a public DNS server directly,
// instead of relying on OS resolver configuration that Termux doesn't
// provide.
package llmclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// fallbackDNSServers are queried, in order, whenever the custom resolver
// needs to open a connection to look up a name. Two independent public
// resolvers are listed so a single provider outage doesn't reintroduce
// the same failure.
var fallbackDNSServers = []string{"8.8.8.8:53", "1.1.1.1:53"}

// newResilientHTTPClient builds an *http.Client whose dialer resolves
// hostnames via fallbackDNSServers instead of the OS resolver
// configuration, so it keeps working on Termux/Android where
// /etc/resolv.conf does not exist.
func newResilientHTTPClient() *http.Client {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			var lastErr error
			for _, dnsServer := range fallbackDNSServers {
				conn, err := d.DialContext(ctx, network, dnsServer)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, fmt.Errorf("llmclient: all fallback DNS servers unreachable: %w", lastErr)
		},
	}

	dialer := &net.Dialer{
		Timeout:  15 * time.Second,
		Resolver: resolver,
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{Transport: transport}
}

// sharedHTTPClient is used by both CallBlocking and CallStreaming in
// place of http.DefaultClient.
var sharedHTTPClient = newResilientHTTPClient()
