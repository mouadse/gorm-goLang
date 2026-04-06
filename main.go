package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"fitness-tracker/api"
	"fitness-tracker/database"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
	if err := api.ValidateJWTConfig(); err != nil {
		log.Fatalf("auth configuration invalid: %v", err)
	}

	port := getEnvOrDefault("PORT", "8080")
	addr := fmt.Sprintf(":%s", port)

	server := api.NewServer(db)
	server.StartBackgroundTasks()
	log.Printf("fitness-tracker API listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func getEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
