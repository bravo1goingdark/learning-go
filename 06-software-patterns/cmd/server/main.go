package main

import (
	"context"
	"log"
	"net/http"

	"learning-go/internal/events"
	"learning-go/internal/events/handlers"
	"learning-go/internal/handler"
	"learning-go/internal/middleware"
	"learning-go/internal/repository"
	"learning-go/internal/service"
)

type contextKey string

const userIDKey contextKey = "userID"

func main() {
	eb := events.New()
	eb.Subscribe("user.created", handlers.NewLoggerHandler())
	eb.Subscribe("user.updated", handlers.NewLoggerHandler())
	eb.Subscribe("user.deleted", handlers.NewLoggerHandler())

	userRepo := repository.NewInMemory()
	userSvc := service.New(userRepo, eb)
	userHandler := handler.New(userSvc)

	mux := http.NewServeMux()

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userHandler.List(w, r)
		case http.MethodPost:
			userHandler.Create(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		id := path[len("/users/"):]
		ctx := context.WithValue(r.Context(), handler.UserIDKey, id)
		r = r.WithContext(ctx)

		switch r.Method {
		case http.MethodGet:
			userHandler.Get(w, r)
		case http.MethodPut:
			userHandler.Update(w, r)
		case http.MethodDelete:
			userHandler.Delete(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	wrapped := middleware.Logger(middleware.Recovery(mux))

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", wrapped))
}
