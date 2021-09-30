package httputil

import "net/http"

func CopyHeaders(w http.ResponseWriter, headers http.Header) {
	for key, values := range headers {
		for _, value := range values {
			w.Header().Set(key, value)
		}
	}
}
