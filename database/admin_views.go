package database

import (
	"log"

	"gorm.io/gorm"
)

// EnsureAdminViews is retained for migration compatibility.
// Admin analytics now query base tables through GORM, so no database views are created.
func EnsureAdminViews(db *gorm.DB) error {
	_ = db
	log.Println("skipping admin dashboard views; analytics use ORM queries")
	return nil
}

// RefreshAdminViews is retained for worker compatibility.
// Admin analytics now query base tables through GORM, so there is nothing to refresh.
func RefreshAdminViews(db *gorm.DB) error {
	_ = db
	return nil
}
