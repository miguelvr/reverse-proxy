package reverse_proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReverseProxy_ServeHTTP(t *testing.T) {
	type request struct {
		method  string
		headers http.Header
		body    []byte
	}

	type response struct {
		statusCode int
		headers    http.Header
		body       []byte
	}

	tests := []struct {
		id       string
		request  request
		expected response
	}{
		{
			id: "GET",
			request: request{
				method: http.MethodGet,
			},
			expected: response{
				statusCode: 200,
			},
		},
		{
			id: "GET JSON",
			request: request{
				method: http.MethodGet,
			},
			expected: response{
				statusCode: 200,
				headers:    http.Header{"Content-Type": []string{"application/json"}},
				body:       []byte("{}"),
			},
		},
		{
			id: "POST",
			request: request{
				method: http.MethodPost,
			},
			expected: response{
				statusCode: 200,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tt.expected.headers != nil {
						w.Header().Set("Content-Type", tt.expected.headers.Get("Content-Type"))
					}
					w.WriteHeader(tt.expected.statusCode)
					_, err := w.Write(tt.expected.body)
					if err != nil {
						t.Fatal("failed to write test server body")
					}
				}),
			)
			defer server.Close()

			serverURL, _ := url.Parse(server.URL)
			reverseProxy := &ReverseProxy{
				target: serverURL,
				client: http.DefaultClient,
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(tt.request.method, server.URL, bytes.NewReader(tt.request.body))

			for key, values := range tt.request.headers {
				for _, value := range values {
					r.Header.Set(key, value)
				}
			}

			reverseProxy.ServeHTTP(w, r)

			require.Equal(t, tt.expected.statusCode, w.Code)
			for key := range tt.expected.headers {
				require.Equal(t, tt.expected.headers.Get(key), w.Header().Get(key))
			}
			require.Equal(t, tt.expected.body, w.Body.Bytes())
		})
	}
}
