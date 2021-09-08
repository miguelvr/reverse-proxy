package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"time"
)

func jsonHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, err := io.WriteString(w, "{}")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func streamHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Transfer-Encoding", "chunked")
	w.(http.Flusher).Flush()

	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		_, err := io.WriteString(w, "hello")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.(http.Flusher).Flush()
	}
}

func trailerHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Trailer", "AtEnd1, AtEnd2")
	w.Header().Add("Trailer", "AtEnd3")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8") // normal header
	w.WriteHeader(http.StatusOK)

	w.Header().Set("AtEnd1", "value 1")
	_, err := io.WriteString(w, "This HTTP response has both headers before this text and trailers at the end.\n")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("AtEnd2", "value 2")
	w.Header().Set("AtEnd3", "value 3") // These will appear as trailers.

}

func main() {
	var port string
	flag.StringVar(&port, "port", "8000", "server port")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/json", jsonHandler)
	mux.HandleFunc("/stream", streamHandler)
	mux.HandleFunc("/trailer", trailerHandler)

	log.Printf("Running on port :%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
