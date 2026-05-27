package publib

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// GetDB opens a connection to the SQLite database
func GetDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logger.Printf("ERROR: Failed to open database: %v\n", err)
		return nil, err
	}

	if err := db.Ping(); err != nil {
		logger.Printf("ERROR: Failed to ping database: %v\n", err)
		return nil, err
	}

	logger.Println("Connected to database successfully")
	return db, nil
}
