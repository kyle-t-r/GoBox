package publib

import (
	"database/sql"
	"log"
	"os"
	"time"
)

var logger = log.New(os.Stdout, "[PubLib] ", log.LstdFlags)

type Event struct {
	Level   string
	Time    int64
	Service string
	Message string
}

func InsertEvent(db *sql.DB, event Event) error {
	_, err := db.Exec("INSERT INTO events (level, time, service, message) VALUES (?, ?, ?, ?)", event.Level, event.Time, event.Service, event.Message)
	if err != nil {
		logger.Printf("ERROR: Failed to insert event - Service: %s, Error: %v\n", event.Service, err)
		return err
	}
	logger.Printf("[EVENT] Created - Service: %s, Message: %s\n", event.Service, event.Message)
	return nil
}

// GetCurrentTime returns the current Unix timestamp
func GetCurrentTime() int64 {
	return time.Now().Unix()
}
