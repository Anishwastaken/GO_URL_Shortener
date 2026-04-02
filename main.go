package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type PageData struct {
	ShortURL string
}

var tmpl *template.Template
var urlStore = make(map[string]string)
var mu sync.RWMutex

const storeFile = "urls.json"

func loadStore() {
	data, err := os.ReadFile(storeFile)
	if err != nil {
		return // file doesn't exist yet, start fresh
	}
	mu.Lock()
	defer mu.Unlock()
	json.Unmarshal(data, &urlStore)
}

func saveStore() {
	mu.RLock()
	defer mu.RUnlock()
	data, err := json.Marshal(urlStore)
	if err != nil {
		fmt.Println("Error saving store:", err)
		return
	}
	os.WriteFile(storeFile, data, 0644)
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateCode(length int) string {
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	return string(code)
}

func startsWithHTTP(url string) bool {
	return len(url) >= 7 && (url[:7] == "http://" || url[:8] == "https://")
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}

func main() {
	rand.Seed(time.Now().UnixNano())
	loadStore()

	tmpl = template.Must(template.ParseGlob("templates/*.html"))

	router := http.NewServeMux()

	
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + getPort() + "/"
	}

	// Home page
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("short")

		var data PageData
		if code != "" {
			data.ShortURL = baseURL + code
		}

		tmpl.ExecuteTemplate(w, "index.html", data)
	})

	// Shorten URL
	router.HandleFunc("/shorten", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request", http.StatusMethodNotAllowed)
			return
		}

		longURL := r.FormValue("url")

		if !startsWithHTTP(longURL) {
		longURL = "https://" + longURL
		}

		if longURL == "" {
			http.Error(w, "URL cannot be empty", http.StatusBadRequest)
			return
		}

		_, err := url.ParseRequestURI(longURL)
		if err != nil {
			http.Error(w, "Invalid URL format", http.StatusBadRequest)
			return
		}

		var code string
		mu.Lock()
		for {
			code = generateCode(6)
			if _, exists := urlStore[code]; !exists {
				break
			}
		}
		urlStore[code] = longURL
		mu.Unlock()
		saveStore()

		http.Redirect(w, r, "/?short="+code, http.StatusSeeOther)
	})

	// Redirect handler
	// Health check for keep-alive pinging
	router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	router.HandleFunc("/{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		mu.RLock()
		longURL, ok := urlStore[code]
		mu.RUnlock()
		if !ok {
			http.NotFound(w, r)
			return
		}

		http.Redirect(w, r, longURL, http.StatusFound)
	})

	srv := http.Server{
		Addr:    ":" + getPort(),
		Handler: router,
	}

	fmt.Println("Server running on port", getPort())

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Println("Error:", err)
	}
}