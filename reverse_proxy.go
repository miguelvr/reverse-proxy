package reverse_proxy

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

// XForwardedForHeader is the key for the X-Forwarded-For http header
const XForwardedForHeader = "X-Forwarded-For"

// ReverseProxy is a reverse proxy that satisfies the net/http Handler interface
type ReverseProxy struct {
	target *url.URL
	client *http.Client
}

// New creates a new instance of ReverseProxy
func New(target *url.URL) *ReverseProxy {
	return &ReverseProxy{
		target: target,
		client: &http.Client{},
	}
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Host = p.target.Host
	r.URL.Host = p.target.Host
	r.URL.Scheme = p.target.Scheme
	r.RequestURI = "" // client does not allow request to have a set request URI

	remoteAddrHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		p.errorHandler(w, err)
		return
	}
	r.Header.Set(XForwardedForHeader, remoteAddrHost)

	resp, err := p.client.Do(r)
	if err != nil {
		p.errorHandler(w, err)
		return
	}

	stop := p.startFlushing(w)
	defer stop()

	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		p.errorHandler(w, err)
		return
	}

	copyHeaders(w, resp.Header)
}

func (p *ReverseProxy) startFlushing(w http.ResponseWriter) func() {
	stopCh := make(chan struct{})
	stopFn := func() {
		close(stopCh)
	}

	go func() {
		for {
			select {
			case <-time.Tick(10 * time.Millisecond):
				w.(http.Flusher).Flush()
			case <-stopCh:
				return
			}
		}
	}()

	return stopFn
}

func (p *ReverseProxy) errorHandler(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadGateway)
	log.Printf("error: %v", err)
}

func copyHeaders(w http.ResponseWriter, headers http.Header) {
	for key, values := range headers {
		for _, value := range values {
			w.Header().Set(key, value)
		}
	}
}
