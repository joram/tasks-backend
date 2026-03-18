package database

import (
	"crypto/rand"
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"task-tracker-api/internal/config"
	"task-tracker-api/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm/logger"
)

const seedSaltSize = 16

func Connect(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.TaskList{},
		&models.Task{},
		&models.ListMember{},
		&models.Label{},
	); err != nil {
		return err
	}
	// Drop legacy unique index on labels(slug) so slug is unique per user, not globally.
	if err := db.Exec(`DROP INDEX IF EXISTS idx_labels_slug`).Error; err != nil {
		log.Printf("migration drop idx_labels_slug: %v", err)
	}
	FlattenTasksWithParentAndChildren(db)
	return nil
}

// FlattenTasksWithParentAndChildren clears parent_id on any task that has both a parent and children,
// so they become root-level (historical data fix).
func FlattenTasksWithParentAndChildren(db *gorm.DB) {
	res := db.Exec(`
		UPDATE tasks t SET parent_id = NULL, updated_at = NOW()
		WHERE t.parent_id IS NOT NULL
		AND EXISTS (SELECT 1 FROM tasks c WHERE c.parent_id = t.id)
	`)
	if res.Error != nil {
		log.Printf("migration FlattenTasksWithParentAndChildren: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("migration: cleared parent_id on %d task(s) that had both parent and children", res.RowsAffected)
	}
}

// SeedDefaultList ensures admin user exists, has a default list, and any tasks with null list_id are assigned to it.
func SeedDefaultList(db *gorm.DB, cfg *config.Config) {
	var admin models.User
	if err := db.Where("email = ?", cfg.AdminEmail).First(&admin).Error; err != nil {
		salt := make([]byte, seedSaltSize)
		if _, err := rand.Read(salt); err != nil {
			log.Printf("seed: failed to generate salt: %v", err)
			return
		}
		input := append(salt, []byte(cfg.AdminPassword)...)
		hash, err := bcrypt.GenerateFromPassword(input, bcrypt.DefaultCost)
		if err != nil {
			log.Printf("seed: failed to hash admin password: %v", err)
			return
		}
		admin = models.User{Email: cfg.AdminEmail, Salt: salt, PasswordHash: string(hash)}
		if err := db.Create(&admin).Error; err != nil {
			log.Printf("seed: failed to create admin user: %v", err)
			return
		}
	}

	var defaultList models.TaskList
	if err := db.Where("user_id = ?", admin.ID).First(&defaultList).Error; err != nil {
		defaultList = models.TaskList{UserID: admin.ID, Name: "Default"}
		if err := db.Create(&defaultList).Error; err != nil {
			log.Printf("seed: failed to create default list: %v", err)
			return
		}
	}

	res := db.Model(&models.Task{}).Where("list_id IS NULL").Update("list_id", defaultList.ID)
	if res.Error != nil {
		log.Printf("seed: failed to assign tasks to default list: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("seed: assigned %d task(s) to default list", res.RowsAffected)
	}
}

// Default label definitions (slug, name, color) for seeding.
var defaultLabelDefs = []struct{ Slug, Name, Color string }{
	{"start_with", "Start With", "#6366F1"},
	{"potential_user", "Potential User", "#06B6D4"},
	{"outreach", "Outreach", "#10B981"},
	{"features", "Features", "#F59E0B"},
	{"house", "House", "#EC4899"},
}

// SeedDefaultLabels assigns existing labels to admin (user 1), then ensures admin has default labels.
// Call after SeedDefaultList so admin user exists.
func SeedDefaultLabels(db *gorm.DB, cfg *config.Config) {
	var admin models.User
	if err := db.Where("email = ?", cfg.AdminEmail).First(&admin).Error; err != nil {
		return
	}
	// Backfill: any labels with empty user_id become owned by admin (e.g. from before user_id existed).
	res := db.Model(&models.Label{}).Where("user_id IS NULL OR user_id = ''").Update("user_id", admin.ID)
	if res.Error == nil && res.RowsAffected > 0 {
		log.Printf("seed: assigned %d label(s) to admin user", res.RowsAffected)
	}
	// Ensure admin has default labels (create any missing).
	for _, d := range defaultLabelDefs {
		var exist models.Label
		if err := db.Where("user_id = ? AND slug = ?", admin.ID, d.Slug).First(&exist).Error; err == nil {
			continue
		}
		label := models.Label{UserID: admin.ID, Slug: d.Slug, Name: d.Name, Color: d.Color}
		if err := db.Create(&label).Error; err != nil {
			log.Printf("seed: failed to create label %s: %v", d.Slug, err)
		}
	}
}
