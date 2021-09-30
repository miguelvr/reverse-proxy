package proxy

import (
	"bytes"
	"io/ioutil"
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
					// verify body is received correctly
					body, err := ioutil.ReadAll(r.Body)
					require.NoError(t, err)
					defer func() { _ = r.Body.Close() }()

					if tt.request.body != nil {
						require.Equal(t, tt.request.body, body)
					}

					// verify headers are received correctly
					for key := range tt.request.headers {
						require.Equal(t, tt.request.headers.Get(key), r.Header.Get(key))
					}

					// set content type
					if tt.expected.headers != nil {
						w.Header().Set("Content-Type", tt.expected.headers.Get("Content-Type"))
					}

					// set status code
					w.WriteHeader(tt.expected.statusCode)

					// write response body
					_, err = w.Write(tt.expected.body)
					require.NoError(t, err)
				}),
			)
			defer server.Close()

			serverURL, _ := url.Parse(server.URL)
			reverseProxy := &ReverseProxy{
				target:        serverURL,
				client:        http.DefaultClient,
				flushInterval: defaultFlushInterval,
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
