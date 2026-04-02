// header-forwarder — transparent HTTP header propagation sidecar.
//
// Listens on two ports:
//
//	:80   — inbound reverse-proxy → captures x-journey-id, forwards to the app on APP_ADDR
//	:9090 — outbound forward-proxy → injects the captured x-journey-id into every request
//
// No application code change is required. The application only needs
// HTTP_PROXY=http://localhost:9090 set in its environment so that
// Go's default HTTP client routes outbound calls through this proxy.
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"time"
)

var (
	mu        sync.RWMutex
	journeyID string // last captured x-journey-id (nil-safe: empty string = absent)
	appAddr   = getEnv("APP_ADDR", "127.0.0.1:8080")
)

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// directTransport never uses HTTP_PROXY so the sidecar does not loop back to itself.
var directTransport = &http.Transport{
	Proxy: nil,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// hopByHop headers must be removed before forwarding.
var hopByHop = []string{
	"Connection", "Proxy-Connection", "Keep-Alive",
	"Proxy-Authenticate", "Proxy-Authorization",
	"Te", "Trailers", "Transfer-Encoding", "Upgrade",
}

func removeHopByHop(h http.Header) {
	for _, name := range hopByHop {
		h.Del(name)
	}
}

// startInboundProxy — captures x-journey-id from every incoming request and
// reverse-proxies the traffic to the application container.
func startInboundProxy() {
	appURL, err := url.Parse("http://" + appAddr)
	if err != nil {
		log.Fatalf("invalid APP_ADDR: %v", err)
	}

	rp := httputil.NewSingleHostReverseProxy(appURL)
	rp.Transport = directTransport

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("x-journey-id")

		mu.Lock()
		journeyID = id
		mu.Unlock()

		if id != "" {
			log.Printf("[inbound] captured x-journey-id=%q  path=%s", id, r.URL.Path)
		} else {
			log.Printf("[inbound] no x-journey-id  path=%s", r.URL.Path)
		}

		rp.ServeHTTP(w, r)
	})

	log.Println("Inbound proxy  :80  →  app at", appAddr)
	if err := http.ListenAndServe(":80", mux); err != nil {
		log.Fatalf("inbound server: %v", err)
	}
}

// startOutboundProxy — acts as an HTTP forward proxy.
// Reads the stored x-journey-id and injects it into every outgoing request.
func startOutboundProxy() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		id := journeyID
		mu.RUnlock()

		// Build a clean outbound request
		outReq := r.Clone(r.Context())
		outReq.RequestURI = "" // must be empty for client use

		// Ensure the URL is absolute (HTTP proxy requests always have an absolute URL)
		if outReq.URL.Scheme == "" {
			outReq.URL.Scheme = "http"
		}
		if outReq.URL.Host == "" {
			outReq.URL.Host = r.Host
		}

		removeHopByHop(outReq.Header)

		if id != "" {
			outReq.Header.Set("x-journey-id", id)
			log.Printf("[outbound] inject x-journey-id=%q  → %s", id, outReq.URL)
		} else {
			outReq.Header.Del("x-journey-id")
			log.Printf("[outbound] no x-journey-id to inject  → %s", outReq.URL)
		}

		resp, err := directTransport.RoundTrip(outReq)
		if err != nil {
			log.Printf("[outbound] upstream error: %v", err)
			http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		removeHopByHop(resp.Header)
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	log.Println("Outbound proxy :9090  (forward proxy for application outbound traffic)")
	if err := http.ListenAndServe(":9090", mux); err != nil {
		log.Fatalf("outbound server: %v", err)
	}
}

func main() {
	fmt.Println("Header-Forwarder sidecar starting…")
	go startInboundProxy()
	startOutboundProxy()
}
