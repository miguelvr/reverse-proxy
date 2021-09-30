package cache

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/miguelvr/reverse-proxy/pkg/httputil"
	"github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type cachedResponse struct {
	Headers http.Header
	Body    []byte
}

type ResponseMultiWriter struct {
	copy  io.Writer
	resp  http.ResponseWriter
	multi io.Writer
}

func NewResponseMultiWriter(copy io.Writer, resp http.ResponseWriter) http.ResponseWriter {
	multi := io.MultiWriter(copy, resp)
	return &ResponseMultiWriter{
		copy:  copy,
		resp:  resp,
		multi: multi,
	}
}

func (w *ResponseMultiWriter) Header() http.Header {
	return w.resp.Header()
}

func (w *ResponseMultiWriter) Write(b []byte) (int, error) {
	return w.multi.Write(b)
}

func (w *ResponseMultiWriter) WriteHeader(i int) {
	w.resp.WriteHeader(i)
}

func (w *ResponseMultiWriter) Flush() {
	w.resp.(http.Flusher).Flush()
}

type Cache struct {
	cache *cache.Cache
	next  http.Handler
}

func WithCache(next http.Handler) http.Handler {
	return &Cache{
		cache: cache.New(1*time.Minute, 10*time.Minute),
		next:  next,
	}
}

func (c *Cache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var cacheAccessSpan trace.Span
	tracer := otel.Tracer("middleware/cache")

	shouldCache := c.shouldCache(r)

	var (
		reqHash string
		err     error
	)

	if shouldCache {
		_, cacheAccessSpan = tracer.Start(ctx, "cache access")
		defer cacheAccessSpan.End()

		// generate a request hash
		reqHash, err = c.requestHash(r)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			log.Printf("error: %v", err)
			return
		}

		// check if the request hash is in the cache
		cachedResp, found := c.getFromCache(reqHash)
		if found {
			httputil.CopyHeaders(w, cachedResp.Headers)
			w.Header().Set("X-Proxy-Cached", "true")
			_, _ = w.Write(cachedResp.Body)
			cacheAccessSpan.AddEvent("cache_hit")
			return
		}
		cacheAccessSpan.End()
	}

	w.Header().Set("X-Proxy-Cached", "false")

	var buf bytes.Buffer
	mw := NewResponseMultiWriter(&buf, w)

	c.next.ServeHTTP(mw, r)

	_, span := tracer.Start(ctx, "save to cache")
	defer span.End()

	cachedResp := cachedResponse{
		Headers: mw.Header(),
		Body:    buf.Bytes(),
	}

	c.saveToCache(reqHash, &cachedResp)
}

func (c *Cache) shouldCache(r *http.Request) bool {
	return r.Method == http.MethodGet
}

func (c *Cache) requestHash(r *http.Request) (string, error) {
	h := sha1.New()
	_, err := io.WriteString(h, r.URL.String())
	if err != nil {
		return "", err
	}

	if err = r.Header.Write(h); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (c *Cache) getFromCache(reqHash string) (*cachedResponse, bool) {
	respBytes, found := c.cache.Get(reqHash)
	if !found {
		return nil, found
	}

	return respBytes.(*cachedResponse), found
}

func (c *Cache) saveToCache(reqHash string, cachedResp *cachedResponse) {
	c.cache.Set(reqHash, cachedResp, cache.DefaultExpiration)
}
