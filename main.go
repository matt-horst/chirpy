package main

import (
	"net/http"
	"fmt"
)

func main() {
	mux := http.NewServeMux()

	var fileSystem http.Dir = "."
	fileServer := http.FileServer(fileSystem)
	mux.Handle("/", fileServer)


	server := http.Server {Addr: ":8080", Handler: mux}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
}
