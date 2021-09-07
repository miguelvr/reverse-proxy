package main

import (
	"flag"
	"log"
	"net/http"

	proxy "github.com/miguelvr/reverse-proxy"
)

func main() {
	var port string
	flag.StringVar(&port, "port", "8000", "server port")
	flag.Parse()

	err := http.ListenAndServe(port, proxy.New())
	if err != nil {
		log.Fatal(err)
	}
}
