package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %v, HTTP Version: %v", r.URL.Path, r.Proto)
	})

	fmt.Printf("Listening [0.0.0.0:8080]...\n")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
