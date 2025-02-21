package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type Game struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type Settings struct {
	Game     Game  `json:"game"`
	Betrates []int `json:"betrates"`
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	game := Game{
		Name:        "Blackjack",
		Slug:        "blackjack",
		Description: "The classic casino game",
	}

	settings := Settings{Game: game, Betrates: []int{1, 2, 3, 4, 5}}

	response, ok := json.Marshal(settings)

	if ok != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

type InvalidateRequest struct {
	Route string `json:"route"`
}

func invalidateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var request InvalidateRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Invalid request"}`))
		return
	}

	cache.Delete(request.Route)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Cache invalidated"}`))
}

func main() {
	mux := http.NewServeMux()

	middleware := withMiddleware(withLogging, withCache)
	mux.HandleFunc("GET /{game}/settings", middleware(settingsHandler))
	mux.HandleFunc("POST /invalidate", withLogging(invalidateHandler))

	// OVER 9000 !!!
	log.Fatal(http.ListenAndServe(":9000", mux))
}
