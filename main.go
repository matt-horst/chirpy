package main

import (
	"net/http"
	"fmt"
)

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func main() {
	mux := http.NewServeMux()

	var fileSystem http.Dir = "."
	fileServer := http.FileServer(fileSystem)
	fileServer = http.StripPrefix("/app", fileServer)
	mux.Handle("/app/", fileServer)

	mux.HandleFunc("/healthz", healthzHandler)

	server := http.Server {Addr: ":8080", Handler: mux}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
}
