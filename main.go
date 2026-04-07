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
	"strings"
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
		return
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

func main() {
	rand.Seed(time.Now().UnixNano())
	loadStore()

	tmpl = template.Must(template.ParseGlob("templates/*.html"))

	router := http.NewServeMux()

	// Home page
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("short")

		var data PageData
		if code != "" {
			// Construct baseURL from request headers
			scheme := "https"
			if strings.Contains(r.Host, "localhost") || strings.Contains(r.Host, "127.0.0.1") {
				scheme = "http"
			}
			baseURL := scheme + "://" + r.Host + "/"
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

		if longURL == "" {
			http.Error(w, "URL cannot be empty", http.StatusBadRequest)
			return
		}

		//  Proper URL handling
		parsed, err := url.Parse(longURL)
		if err != nil || parsed.Host == "" {
			longURL = "https://" + longURL
			parsed, err = url.Parse(longURL)
			if err != nil || parsed.Host == "" {
				http.Error(w, "Invalid URL", http.StatusBadRequest)
				return
			}
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
	router.HandleFunc("/{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		mu.RLock()
		longURL, ok := urlStore[code]
		mu.RUnlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		//  DEBUG (important)
		fmt.Println("Redirecting to:", longURL)

		http.Redirect(w, r, longURL, http.StatusFound)
	})

	srv := http.Server{
		Addr:    ":" + getPort(),
		Handler: router,
	}

	fmt.Println("Running on port", getPort())

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Println("Error:", err)
	}
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}
