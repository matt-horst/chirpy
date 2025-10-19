package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileServerHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Add("Content-Type", "text/html;charset=utf-8")
	w.WriteHeader(200)
	html := `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`
	_, err := w.Write(fmt.Appendf([]byte{}, html, cfg.fileServerHits.Load()))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, _ *http.Request) {
	cfg.fileServerHits.Store(0)
	w.WriteHeader(200)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	chirp := struct {
		Body string `json:"body"`
	} {}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&chirp)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		w.WriteHeader(500)
		return
	}

	if len(chirp.Body) <= 140 {
		profanity := []string{"kerfuffle", "sharbert", "fornax"}
		words := strings.Split(chirp.Body, " ")
		for i, word := range words {
			if slices.Contains(profanity[:], strings.ToLower(word)) {
				words[i] = "****"
			}
		}

		cleaned := struct {
			CleanedBody string `json:"cleaned_body"`
		} {CleanedBody: strings.Join(words, " ")}

		responsdWithJson(w, 200, cleaned)
	} else {
		respondWithError(w, 400, "Chirp is too long")
	}
}

func responsdWithJson(w http.ResponseWriter, code int, payload interface{}) {
		resp, err := json.Marshal(payload)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			w.WriteHeader(500)
			return
		}
		
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(code)
		w.Write(resp)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
		errorMsg := struct {
			Error string `json:"error"`
		} {Error: msg}

		resp, err := json.Marshal(errorMsg)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			w.WriteHeader(500)
			return
		}
		
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(code)
		w.Write(resp)
}


func main() {
	mux := http.NewServeMux()

	apiConfig := apiConfig{}

	var fileSystem http.Dir = "."
	fileServer := http.FileServer(fileSystem)
	fileServer = http.StripPrefix("/app", fileServer)
	mux.Handle("/app/", apiConfig.middlewareMetricsInc(fileServer))

	mux.HandleFunc("GET /api/healthz", healthzHandler)
	mux.HandleFunc("GET /admin/metrics", apiConfig.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiConfig.resetHandler)
	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)

	server := http.Server {Addr: ":8080", Handler: mux}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
}
