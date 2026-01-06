package database

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dataDir string) *gorm.DB {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal("Failed to create data directory:", err)
	}

	dbPath := filepath.Join(dataDir, "bills.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get database handle:", err)
	}

	// SQLite tuning:
	// - keep a single connection so PRAGMA settings apply consistently
	// - reduce "database is locked" under concurrent requests
	// - improve read/write concurrency with WAL
	applySQLiteTuning(sqlDB)

	DB = db
	return db
}

func GetDB() *gorm.DB {
	return DB
}

func applySQLiteTuning(sqlDB *sql.DB) {
	if sqlDB == nil {
		return
	}

	// One connection avoids per-connection PRAGMA drift and reduces lock contention.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA busy_timeout = 5000;",
		// 20MB page cache (negative value means KiB).
		"PRAGMA cache_size = -20000;",
		// Auto-checkpoint after 1000 pages (~4MB with default page_size=4096).
		"PRAGMA wal_autocheckpoint = 1000;",
	}

	for _, q := range pragmas {
		if _, err := sqlDB.Exec(q); err != nil {
			log.Printf("[DB] sqlite pragma failed: %s err=%v", q, err)
		}
	}
}
