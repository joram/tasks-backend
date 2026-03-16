package main

import (
	"log"

	"task-tracker-api/internal/config"
	"task-tracker-api/internal/database"
	"task-tracker-api/internal/router"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	database.SeedDefaultList(db, cfg)
	database.SeedDefaultLabels(db, cfg)

	r := router.Setup(db, cfg)

	log.Printf("server starting on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
