package main

import (
	"errors"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"time"
)

type PageData struct {
	ShortURL string
}

var tmpl *template.Template
var urlStore = make(map[string]string)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateCode(length int) string {
	rand.Seed(time.Now().UnixNano())
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	return string(code)
}

func main() {
	tmpl = template.Must(template.ParseGlob("templates/*.html"))

	router := http.NewServeMux()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("short")

		var data PageData
		if code != "" {
			data.ShortURL = "http://localhost:8080/" + code
		}

		tmpl.ExecuteTemplate(w, "index.html", data)
	})

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

		var code string
		for {
			code = generateCode(6)
			if _, exists := urlStore[code]; !exists {
				break
			}
		}

		urlStore[code] = longURL

		http.Redirect(w, r, "/?short="+code, http.StatusSeeOther)
	})

	router.HandleFunc("/{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		longURL, ok := urlStore[code]
		if !ok {
			http.NotFound(w, r)
			return
		}

		http.Redirect(w, r, longURL, http.StatusFound)
	})

	srv := http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	fmt.Println("Running at http://localhost:8080")

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Println("Error:", err)
	}
}