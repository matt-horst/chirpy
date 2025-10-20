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
	"github.com/matt-horst/chirpy/internal/auth"
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

func responsdWithJson(w http.ResponseWriter, code int, payload any) {
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

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Password string `json:"password"`
		Email string 	`json:"email"`
	} {}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		w.WriteHeader(500)
		return
	}

	hashed_password, err := auth.HashPassword(data.Password)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		w.WriteHeader(500)
		return
	}

	params := database.CreateUserParams {Email: data.Email, HashedPassword: hashed_password}
	dbUser, err := cfg.dbQueries.CreateUser(r.Context(), params)
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

func (cfg *apiConfig) loginUserHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Password string `json:"password"`
		Email string 	`json:"email"`
	} {}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&data)
	if err != nil {
		w.WriteHeader(500)
		fmt.Printf("Error: %v\n", err)
		return
	}

	user, err := cfg.dbQueries.GetUser(r.Context(), data.Email)
	if err != nil {
		respondWithError(w, 401, "Incorrect email and password")
	}

	ok, err := auth.CheckPasswordHash(data.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, 401, "Incorrect email and password")
	}

	if ok {
		resp := User {
			ID: user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email: user.Email,
		}
		responsdWithJson(w, 200, resp)
	} else {
		respondWithError(w, 401, "Incorrect email and password")
	}
}

func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	chirp := struct {
		Body string 		`json:"body"`
		UserID uuid.UUID 	`json:"user_id"`
	} {}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&chirp)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		w.WriteHeader(500)
		return
	}

	if len(chirp.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
	}

	profanity := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Split(chirp.Body, " ")
	for i, word := range words {
		if slices.Contains(profanity[:], strings.ToLower(word)) {
			words[i] = "****"
		}
	}

	params := database.CreateChirpParams {
		Body: strings.Join(words, " "),
		UserID: uuid.NullUUID { Valid: true, UUID: chirp.UserID },
	}
	dbChirp, err := cfg.dbQueries.CreateChirp(r.Context(), params)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		w.WriteHeader(500)
		return
	}

	resp := Chirp {
		ID: dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body: dbChirp.Body,
		UserID: dbChirp.UserID.UUID,
	}

	responsdWithJson(w, 201, resp)
}

func (cfg *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		w.WriteHeader(500)
		fmt.Printf("Error: %v\n", err)
		return
	}

	var resp []Chirp
	for _, chirp := range chirps {
		resp = append(resp, Chirp{
			ID: chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body: chirp.Body,
			UserID: chirp.UserID.UUID,
		})
	}

	responsdWithJson(w, 200, resp)
}

func (cfg *apiConfig) getSingleChirpHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		w.WriteHeader(500)
		fmt.Printf("Error: %v\n", err)
		return
	}

	chirp, err := cfg.dbQueries.GetSingleChirp(r.Context(), id)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	resp := Chirp {
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID.UUID,
	}

	responsdWithJson(w, 200, resp)
}

type User struct {
	ID uuid.UUID 		`json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email string 		`json:"email"`
};

type Chirp struct {
	ID uuid.UUID		`json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body string			`json:"body"`
	UserID uuid.UUID 	`json:"user_id"`
}


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
	mux.HandleFunc("POST /api/users", apiConfig.createUserHandler)
	mux.HandleFunc("POST /api/login", apiConfig.loginUserHandler)
	mux.HandleFunc("POST /api/chirps", apiConfig.createChirpHandler)
	mux.HandleFunc("GET /api/chirps", apiConfig.getAllChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiConfig.getSingleChirpHandler)

	server := http.Server {Addr: ":8080", Handler: mux}

	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
}
