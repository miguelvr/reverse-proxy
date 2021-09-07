package reverse_proxy

import "net/http"

// ReverseProxy is a reverse proxy that satisfies the net/http Handler interface
type ReverseProxy struct {
	client http.Client
}

// New creates a new instance of ReverseProxy
func New() *ReverseProxy {
	return &ReverseProxy{
		client: http.Client{},
	}
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	panic("implement me")
}
