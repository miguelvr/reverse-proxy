package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"

	proxy "github.com/miguelvr/reverse-proxy"
)

func main() {
	var (
		target string
		port   string
	)
	flag.StringVar(&target, "target-url", "", "target url where the traffic will be forwarded to")
	flag.StringVar(&port, "port", "8000", "server port")
	flag.Parse()

	if target == "" {
		log.Fatal("--target-url flag is required")
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		log.Fatal(err)
	}

	reverseProxy := proxy.New(targetURL)
	cachedReverseProxy := proxy.NewCache(reverseProxy)

	log.Printf("Running on port :%s\n", port)
	if err = http.ListenAndServe(":"+port, cachedReverseProxy); err != nil {
		log.Fatal(err)
	}
}
