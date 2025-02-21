package main

import (
	"log"
	"net/http"
	"sync"
	"time"
)

type CacheEntry struct {
	Body         []byte
	Headers      http.Header
	ExpiresAt    time.Time
	RevalidateAt time.Time
}

var (
	cache         = sync.Map{}
	cacheTTL      = 10 * time.Minute
	revalidateTTL = 5 * time.Minute
)

type recordingResponseWriter struct {
	http.ResponseWriter
	Body    []byte
	Headers http.Header
	Code    int
}

func (w *recordingResponseWriter) Write(b []byte) (int, error) {
	w.Body = b
	return w.ResponseWriter.Write(b)
}

func (w *recordingResponseWriter) WriteHeader(statusCode int) {
	w.Code = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
	w.Headers = w.ResponseWriter.Header()
}

type noopResponseWriter struct{}

func (w *noopResponseWriter) Header() http.Header {
	return make(http.Header)
}

func (w *noopResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (w *noopResponseWriter) WriteHeader(int) {}

func revalidate(cacheKey string, r *http.Request, next http.HandlerFunc) {
	r2 := r.Clone(r.Context())
	response := &recordingResponseWriter{ResponseWriter: &noopResponseWriter{}}
	next(response, r2)

	if response.Code == http.StatusOK {
		// Update the cache with the recorded response
		cacheEntry := CacheEntry{
			Body:         response.Body,
			Headers:      response.Headers,
			ExpiresAt:    time.Now().Add(cacheTTL),
			RevalidateAt: time.Now().Add(revalidateTTL),
		}
		cache.Store(cacheKey, cacheEntry)
		log.Printf("Revalidation successful for %s", cacheKey)
	} else {
		// Extend the revalidation time to avoid hammering the backend
		if entry, ok := cache.Load(cacheKey); ok {
			cacheEntry := entry.(CacheEntry)
			cacheEntry.RevalidateAt = time.Now().Add(revalidateTTL * 2)
			cache.Store(cacheKey, cacheEntry)
		}
		log.Printf("Revalidation failed for %s, status code: %d", cacheKey, response.Code)
	}
}

func withCache(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cacheKey := r.URL.Path
		if entry, ok := cache.Load(cacheKey); ok {
			cacheEntry := entry.(CacheEntry)

			// Hit
			if cacheEntry.ExpiresAt.After(time.Now()) {
				log.Printf("Cache hit for %s", cacheKey)

				// Add headers
				for key, values := range cacheEntry.Headers {
					for _, value := range values {
						w.Header().Set(key, value)
					}
				}

				w.Header().Set("X-Cache", "HIT")
				w.Write(cacheEntry.Body)
				return
			}

			// Stale
			if cacheEntry.RevalidateAt.Before(time.Now()) {
				log.Printf("Cache stale for %s", cacheKey)

				// Add headers
				for key, values := range cacheEntry.Headers {
					for _, value := range values {
						w.Header().Set(key, value)
					}
				}

				w.Header().Set("X-Cache", "STALE")
				w.Write(cacheEntry.Body)
				go revalidate(cacheKey, r, next)
				return
			}

			// Expired
			cache.Delete(cacheKey)
		}

		// Miss
		log.Printf("Cache miss for %s", cacheKey)
		w.Header().Set("X-Cache", "MISS")
		response := &recordingResponseWriter{ResponseWriter: w, Headers: make(http.Header)}
		next(response, r)

		if response.Code == http.StatusOK {
			cacheEntry := CacheEntry{
				Body:         response.Body,
				Headers:      response.Headers,
				ExpiresAt:    time.Now().Add(cacheTTL),
				RevalidateAt: time.Now().Add(revalidateTTL),
			}
			cache.Store(cacheKey, cacheEntry)
		}
	}
}
