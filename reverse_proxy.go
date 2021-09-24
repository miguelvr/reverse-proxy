package reverse_proxy

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
)

const (
	// TrailerHeader is the key for the HTTP trailer header
	TrailerHeader = "Trailer"

	// XForwardedForHeader is the key for the X-Forwarded-For http header
	XForwardedForHeader = "X-Forwarded-For"

	defaultFlushInterval = 10 * time.Millisecond
)

// ReverseProxy is a reverse proxy that satisfies the net/http Handler interface
type ReverseProxy struct {
	target        *url.URL
	client        *http.Client
	cache         *cache.Cache
	flushInterval time.Duration
}

// New creates a new instance of ReverseProxy
func New(target *url.URL) *ReverseProxy {
	return &ReverseProxy{
		target:        target,
		client:        http.DefaultClient,
		cache:         cache.New(1*time.Minute, 10*time.Minute),
		flushInterval: defaultFlushInterval,
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

	var (
		reqHash string
		resp    *http.Response
		found   bool
	)

	// decide if request should be cached, since not all requests should be cached
	shouldCache := p.shouldCache(r)
	if shouldCache {
		// generate a request hash
		reqHash, err = p.requestHash(r)
		if err != nil {
			p.errorHandler(w, err)
			return
		}

		// check if the request hash is in the cache
		resp, found, err = p.getFromCache(reqHash)
		if err != nil {
			log.Printf("error: %v\n", err)
		}
	}

	if !found || !shouldCache {
		// execute http request
		resp, err = p.client.Do(r)
		if err != nil {
			p.errorHandler(w, err)
			return
		}

		// start goroutine to periodically flush request writer
		stop := p.startFlushing(w)
		defer stop()
	}

	// custom header to validate if a response was cached (mostly for demo purposes)
	w.Header().Set("X-Proxy-Cached", fmt.Sprintf("%t", shouldCache && found))

	// Announce trailer header
	trailerKeys := p.getTrailerKeys(resp)
	if len(trailerKeys) > 0 {
		w.Header().Set(TrailerHeader, strings.Join(trailerKeys, ","))
	}

	// copy response headers from target server
	copyHeaders(w, resp.Header)

	// write status code
	w.WriteHeader(resp.StatusCode)

	// set up a tee reader to read into the response writer and to a buffer for caching the response
	var buf bytes.Buffer
	respBodyReader := resp.Body
	if !found && shouldCache {
		respBodyReader = io.NopCloser(io.TeeReader(resp.Body, &buf))
	}

	// guarantee that the request body is always closed
	defer func() { _ = r.Body.Close() }()

	// copy response body from target server
	_, err = io.Copy(w, respBodyReader)
	if err != nil {
		p.errorHandler(w, err)
		return
	}

	// copy trailer header values
	copyHeaders(w, resp.Trailer)

	// cache response if required
	if !found && shouldCache {
		// replace response body with the unread buffer reader
		resp.Body = io.NopCloser(&buf)

		err = p.saveToCache(reqHash, resp)
		if err != nil {
			log.Printf("error: %v\n", err)
		}
	}
}

func (p *ReverseProxy) startFlushing(w http.ResponseWriter) func() {
	stopCh := make(chan struct{})
	stopFn := func() {
		close(stopCh)
	}

	ticker := time.NewTicker(p.flushInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				w.(http.Flusher).Flush()
			case <-stopCh:
				return
			}
		}
	}()

	return stopFn
}

func (p *ReverseProxy) getTrailerKeys(response *http.Response) []string {
	trailerKeys := make([]string, len(response.Trailer))
	var i int
	for key := range response.Trailer {
		trailerKeys[i] = key
		i++
	}
	return trailerKeys
}

func (p *ReverseProxy) errorHandler(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadGateway)
	log.Printf("error: %v", err)
}

func (p *ReverseProxy) shouldCache(r *http.Request) bool {
	return r.Method == http.MethodGet
}

func (p *ReverseProxy) requestHash(request *http.Request) (string, error) {
	h := sha1.New()
	_, err := io.WriteString(h, request.URL.String())
	if err != nil {
		return "", err
	}

	if err = request.Header.Write(h); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (p *ReverseProxy) getFromCache(reqHash string) (*http.Response, bool, error) {
	respBytes, found := p.cache.Get(reqHash)
	if !found {
		return nil, found, nil
	}

	buf := bufio.NewReader(bytes.NewReader(respBytes.([]byte)))
	response, err := http.ReadResponse(buf, nil)
	if err != nil {
		return nil, false, err
	}

	return response, found, nil
}

func (p *ReverseProxy) saveToCache(reqHash string, response *http.Response) error {
	buf := &bytes.Buffer{}
	if err := response.Write(buf); err != nil {
		return err
	}

	p.cache.Set(reqHash, buf.Bytes(), cache.DefaultExpiration)
	return nil
}

func copyHeaders(w http.ResponseWriter, headers http.Header) {
	for key, values := range headers {
		for _, value := range values {
			w.Header().Set(key, value)
		}
	}
}
