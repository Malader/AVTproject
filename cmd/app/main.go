package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"AVTproject/config"
	"AVTproject/handlers"
	"AVTproject/repository"
	"AVTproject/service"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func main() {
	ctx := context.Background()

	cfg := config.LoadConfigOrPanic()

	db := config.InitDB(ctx, cfg)
	defer func() { _ = db.Close() }()

	repoImpl := repository.NewPostgresRepository(db)

	svc := service.NewService(repoImpl, cfg.JWTSecret)

	h := handlers.NewHandler(svc, cfg.JWTSecret)

	r := mux.NewRouter()
	r.HandleFunc("/api/auth", h.AuthHandler).Methods("POST")
	r.HandleFunc("/api/info", h.JWTMiddleware(h.InfoHandler)).Methods("GET")
	r.HandleFunc("/api/sendCoin", h.JWTMiddleware(h.SendCoinHandler)).Methods("POST")
	r.HandleFunc("/api/buy/{item}", h.JWTMiddleware(h.BuyHandler)).Methods("GET")

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("Добро пожаловать в Avito Shop API")); err != nil {
			log.Printf("Ошибка при записи ответа: %v", err)
		}
	}).Methods("GET")

	srv := http.Server{
		Handler:      r,
		Addr:         ":" + cfg.ServerPort,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Printf("Сервер запущен на порту %s", cfg.ServerPort)
	if err := srv.ListenAndServe(); err != nil {
		_ = db.Close()
		log.Fatal(err)
	}
}
