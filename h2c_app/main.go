package main

import (
	"fmt"
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	h2s := &http2.Server{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Request %+v\n", r)
		fmt.Printf("Headers %+v\n", r.Header)
		fmt.Printf("Hello, %v, used tls: %v, proto: %v\n", r.URL.Path, r.TLS != nil, r.Proto)
		fmt.Print("\n")
		fmt.Fprintf(w, "Hello, %v, Used TLS: %v", r.URL.Path, r.TLS != nil)
	})

	server := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: h2c.NewHandler(handler, h2s),
	}

	fmt.Printf("Listening [0.0.0.0:8080]...\n")
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
