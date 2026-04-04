package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"fitness-tracker/database"
	"fitness-tracker/services"
)

const defaultDatasetPath = "FoodData_Central_foundation_food_json_2025-12-18.json"

func main() {
	datasetPath := defaultDatasetPath
	if len(os.Args) > 1 {
		datasetPath = os.Args[1]
	}

	if _, err := os.Stat(datasetPath); os.IsNotExist(err) {
		log.Fatalf("dataset file not found: %s\nUsage: go run scripts/import_usda.go [path-to-usda-json]", datasetPath)
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	importer := services.NewUSDAImportService(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("Starting USDA import from: %s", datasetPath)
	stats, err := importer.ImportFromFile(ctx, datasetPath)
	if err != nil {
		log.Fatalf("import failed: %v", err)
	}

	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║      USDA Food Import Complete         ║")
	fmt.Println("╠════════════════════════════════════════╣")
	fmt.Printf("║  Foods imported:  %-20d║\n", stats.FoodCount)
	fmt.Printf("║  New foods:       %-20d║\n", stats.NewFoods)
	fmt.Printf("║  Nutrient rows:   %-20d║\n", stats.NutrientRow)
	fmt.Printf("║  Duration:        %-20s║\n", stats.Duration.Round(time.Millisecond))
	fmt.Println("╚════════════════════════════════════════╝")
}
