# Reverse Proxy

A simple HTTP reverse proxy with request caching, written in GoLang.

## Supported Features

- [x] HTTP Proxying
- [x] HTTP Streaming
- [x] Trailer
- [x] `GET` request caching

## Demo

1) Run the demo server
    ```bash
    go run ./cmd/demo --port 8000
    ```

2) Run the reverse proxy
    ```bash
    go run ./cmd/reverse_proxy --port 8001 --target-url http://localhost:8000
    ```

3) Send requests to the demo server through the request proxy

    ```bash
    # JSON Endpoint:
    curl -i --raw http://localhost:8001/json
    
    # HTTP/1.1 200 OK
    # Content-Length: 2
    # Content-Type: application/json
    # Date: Thu, 09 Sep 2021 18:18:54 GMT
    # X-Proxy-Cached: false
    # 
    # {}
   
    # Streaming Endpoint
    curl -i --raw http://localhost:8001/stream
    
    # HTTP/1.1 200 OK
    # Date: Thu, 09 Sep 2021 18:21:20 GMT
    # X-Proxy-Cached: false
    # Transfer-Encoding: chunked
    # 
    # 5
    # hello
    # 5
    # hello
    # 5
    # hello
    # 5
    # hello
    # 5
    # hello
    # 0
   
    # Trailer Endpoint
    curl -i --raw http://localhost:8001/stream
   
    # HTTP/1.1 200 OK
    # Content-Type: text/plain; charset=utf-8
    # Date: Thu, 09 Sep 2021 18:22:54 GMT
    # Trailer: Atend1,Atend2,Atend3
    # X-Proxy-Cached: false
    # Transfer-Encoding: chunked
    # 
    # 4e
    # This HTTP response has both headers before this text and trailers at the end.
    # 
    # 0
    # Atend1: value 1
    # Atend2: value 2
    # Atend3: value 3
    ```
