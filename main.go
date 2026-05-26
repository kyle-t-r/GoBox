package main

import (
	"database/sql"
	"html/template"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/kyle-t-r/gobox/listeners"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

/**
 * GoBox - Event logging and system monitoring tool
 * @author: kyle-t-r
 * @version: 1.0.0
 * @description: A simple Go application that logs events, including a utility for monitoring system stats.
 */

var (
	logger = log.New(os.Stdout, "[GoBox] ", log.LstdFlags)
)

type Config struct {
	DBConnectionString string
	DBType             string
	CPUThresholdWarn   float64
	CPUThresholdCrit   float64
	MemThresholdWarn   float64
	MemThresholdCrit   float64
	DiskThresholdWarn  float64
	DiskThresholdCrit  float64
	Port               string
}

type Event struct {
	Id      int
	Time    int
	Service string
	Message string
}

var config *Config
var db *sql.DB

func main() {
	logger.Println("========== Starting GoBox ==========")

	getConfig()
	getDB()

	logger.Println("Starting server and listener...")
	go startServer()
	go startListener()

	// Block thread to keep main alive
	select {}
}

func getConfig() {
	logger.Println("[CONFIG] Loading environment...")
	err := godotenv.Load()
	if err != nil {
		logger.Println("[CONFIG] Warning: .env file not found, using system environment")
	}

	config = &Config{
		DBConnectionString: os.Getenv("DB_CONNECTION_STRING"),
		DBType:             os.Getenv("DB_TYPE"),
		CPUThresholdWarn:   parseEnvFloat("CPU_THRESHOLD_WARNING", 50.0),
		CPUThresholdCrit:   parseEnvFloat("CPU_THRESHOLD_CRITICAL", 90.0),
		MemThresholdWarn:   parseEnvFloat("MEM_THRESHOLD_WARNING", 50.0),
		MemThresholdCrit:   parseEnvFloat("MEM_THRESHOLD_CRITICAL", 90.0),
		DiskThresholdWarn:  parseEnvFloat("DISK_THRESHOLD_WARNING", 50.0),
		DiskThresholdCrit:  parseEnvFloat("DISK_THRESHOLD_CRITICAL", 90.0),
		Port:               ":" + os.Getenv("PORT"),
	}

	logger.Printf("[CONFIG] Database Type: %s\n", config.DBType)
	logger.Printf("[CONFIG] Port: %s\n", config.Port)
	logger.Println("[CONFIG] Configuration loaded successfully")
}

func parseEnvFloat(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		logger.Printf("[CONFIG] Warning: invalid float for %s: %v, using default %.2f\n", key, err, defaultValue)
		return defaultValue
	}
	return parsed
}

func getDB() {
	logger.Println("[DATABASE] Opening connection...")
	unsafeDB, err := sql.Open(config.DBType, config.DBConnectionString)
	if err != nil {
		logger.Fatalf("[DATABASE] ERROR: Failed to open database: %v\n", err)
	}

	if err := unsafeDB.Ping(); err != nil {
		logger.Fatalf("[DATABASE] ERROR: Failed to ping database: %v\n", err)
	}

	db = unsafeDB
	logger.Println("[DATABASE] Connected successfully")
	initDB()
}

func initDB() {
	logger.Println("[DATABASE] Initializing tables...")
	var createTableSQL string

	switch config.DBType {
	case "sqlite3":
		createTableSQL = `
        CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            time INTEGER NOT NULL,
            service TEXT NOT NULL,
            message TEXT NOT NULL
        );
        `
	case "mysql":
		createTableSQL = `
        CREATE TABLE IF NOT EXISTS events (
            id INT AUTO_INCREMENT PRIMARY KEY,
            time BIGINT NOT NULL,
            service VARCHAR(255) NOT NULL,
            message TEXT NOT NULL
        );
        `
	}

	if createTableSQL != "" {
		_, err := db.Exec(createTableSQL)
		if err != nil {
			logger.Fatalf("[DATABASE] ERROR: Failed to create events table: %v\n", err)
		}
		logger.Println("[DATABASE] Events table ready")
	}
}

func startListener() {
	logger.Println("[LISTENER] Starting stats listener...")
	thresholds := listeners.Thresholds{
		CPUWarnPercent:      config.CPUThresholdWarn,
		CPUCriticalPercent:  config.CPUThresholdCrit,
		MemWarnPercent:      config.MemThresholdWarn,
		MemCriticalPercent:  config.MemThresholdCrit,
		DiskWarnPercent:     config.DiskThresholdWarn,
		DiskCriticalPercent: config.DiskThresholdCrit,
	}
	logger.Printf("[LISTENER] Thresholds - CPU Warn: %.1f%%, CPU Critical: %.1f%%, Memory Warn: %.1f%%, Memory Critical: %.1f%%, Disk Warn: %.1f%%, Disk Critical: %.1f%%\n",
		thresholds.CPUWarnPercent, thresholds.CPUCriticalPercent, thresholds.MemWarnPercent, thresholds.MemCriticalPercent, thresholds.DiskWarnPercent, thresholds.DiskCriticalPercent)

	listener := listeners.NewStatsListener("http://localhost"+config.Port, thresholds)
	cronJob := listener.Start()
	defer cronJob.Stop()

	select {}
}

func startServer() {
	r := gin.Default()
	r.Static("/static", "./static")
	r.GET("/new", createEvent)
	r.GET("/read", readEvents)

	logger.Printf("[SERVER] Starting on %s\n", config.Port)
	r.Run(config.Port)
}

func createEvent(c *gin.Context) {
	service := c.DefaultQuery("service", "log-service")
	message := c.DefaultQuery("message", "Message not provided. This is a bug.")
	eventTime := time.Now().Unix()

	_, err := db.Exec("INSERT INTO events (time, service, message) VALUES (?, ?, ?)", eventTime, service, message)
	if err != nil {
		logger.Printf("[EVENT] ERROR: Failed to insert event - Service: %s, Error: %v\n", service, err)
		c.JSON(500, gin.H{"error": "Failed to create event"})
		return
	}

	logger.Printf("[EVENT] Created - Service: %s, Message: %s\n", service, message)
	c.JSON(200, gin.H{"status": "Event created"})
}

type PageData struct {
	Entries    []Event
	Limit      string
	Offset     string
	Page       int
	NextOffset int
	PrevOffset int
}

func readEvents(c *gin.Context) {
	limit := c.DefaultQuery("limit", "50")
	offset := c.DefaultQuery("offset", "0")

	logger.Printf("[QUERY] Reading events - Limit: %s, Offset: %s\n", limit, offset)

	query := "SELECT id, time, service, message FROM events ORDER BY time DESC LIMIT ? OFFSET ?"
	rows, err := db.Query(query, limit, offset)
	if err != nil {
		logger.Printf("[QUERY] ERROR: Database query failed: %v\n", err)
		c.String(500, "Database error")
		return
	}
	defer rows.Close()

	entries := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Id, &e.Time, &e.Service, &e.Message); err != nil {
			logger.Printf("[QUERY] ERROR: Failed to scan row: %v\n", err)
			continue
		}
		entries = append(entries, e)
	}

	offsetInt, _ := strconv.Atoi(offset)
	limitInt, _ := strconv.Atoi(limit)

	data := PageData{
		Entries:    entries,
		Limit:      limit,
		Offset:     offset,
		Page:       (offsetInt / limitInt) + 1,
		NextOffset: offsetInt + limitInt,
		PrevOffset: offsetInt - limitInt,
	}

	tmpl := template.New("template.html").Funcs(map[string]interface{}{
		"formatTime": func(unixTime int) string {
			return time.Unix(int64(unixTime), 0).Format("2006-01-02 15:04:05")
		},
	})

	tmpl, err = tmpl.ParseFiles("static/template.html")
	if err != nil {
		logger.Printf("[TEMPLATE] ERROR: Failed to parse template: %v\n", err)
		c.String(500, "Template error")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(c.Writer, data)
}
