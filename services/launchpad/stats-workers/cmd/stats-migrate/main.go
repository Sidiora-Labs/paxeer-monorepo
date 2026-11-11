package main

import (
	"context"
	"log"
	"os"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/migrate"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("stats-migrate: DATABASE_URL is required")
	}
	if err := migrate.Up(context.Background(), dsn); err != nil {
		log.Fatalf("stats-migrate: %v", err)
	}
	log.Print("stats-migrate: migrations applied")
}
