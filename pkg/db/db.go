// Package db provides database connection and utilities
package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"online-game/pkg/config"
)

// DB holds the database connection
type DB struct {
	*gorm.DB
}

// New creates a new database connection
func New(cfg *config.DatabaseConfig) (*DB, error) {
	dsn := cfg.GetDSN()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return &DB{DB: db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping checks if the database connection is alive
func (db *DB) Ping() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Transaction executes a function within a transaction
func (db *DB) Transaction(fn func(tx *gorm.DB) error) error {
	return db.DB.Transaction(fn)
}

// Model returns a GORM model for the given value
func (db *DB) Model(value interface{}) *gorm.DB {
	return db.DB.Model(value)
}

// AutoMigration runs auto migration for given models
func (db *DB) AutoMigration(models ...interface{}) error {
	return db.DB.AutoMigrate(models...)
}
