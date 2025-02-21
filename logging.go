package main

import (
	"log"
	"net/http"
)

func withLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Log: %s %s", r.Method, r.URL.Path)
		next(w, r)
	}
}
