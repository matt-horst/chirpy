package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/matt-horst/chirpy/internal/database"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	dbQueries *database.Queries
	platform string
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

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(403)
	} else {
		cfg.fileServerHits.Store(0)
		cfg.dbQueries.DeleteUsers(r.Context())
		w.WriteHeader(200)
	}
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

func (cfg *apiConfig) usersHandler(w http.ResponseWriter, r *http.Request) {
	email := struct {
		Email string `json:"email"`
	} {}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&email)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		w.WriteHeader(500)
		return
	}

	dbUser, err := cfg.dbQueries.CreateUser(r.Context(), email.Email)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		w.WriteHeader(500)
		return
	}

	user := User {
		ID: dbUser.ID, 
		CreatedAt: dbUser.CreatedAt, 
		UpdatedAt: dbUser.UpdatedAt, 
		Email: dbUser.Email, 
	}

	responsdWithJson(w, 201, user)
}

type User struct {
	ID uuid.UUID 		`json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email string 		`json:"email"`
};


func main() {
	godotenv.Load(".env")
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	dbQueries := database.New(db)

	mux := http.NewServeMux()

	apiConfig := apiConfig{dbQueries: dbQueries, platform: platform}

	var fileSystem http.Dir = "."
	fileServer := http.FileServer(fileSystem)
	fileServer = http.StripPrefix("/app", fileServer)
	mux.Handle("/app/", apiConfig.middlewareMetricsInc(fileServer))

	mux.HandleFunc("GET /api/healthz", healthzHandler)
	mux.HandleFunc("GET /admin/metrics", apiConfig.metricsHandler)
	mux.HandleFunc("POST /admin/reset", apiConfig.resetHandler)
	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
	mux.HandleFunc("POST /api/users", apiConfig.usersHandler)

	server := http.Server {Addr: ":8080", Handler: mux}

	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
}
