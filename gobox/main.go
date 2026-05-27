package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
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

type YAMLConfig struct {
	Database   DatabaseConfig             `yaml:"database"`
	Server     ServerConfig               `yaml:"server"`
	Publishers map[string]PublisherConfig `yaml:"publishers"`
}

type DatabaseConfig struct {
	Type       string `yaml:"type"`
	Connection string `yaml:"connection"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type PublisherConfig struct {
	Schedule   string                 `yaml:"schedule"`
	Executable string                 `yaml:"executable"`
	Config     map[string]interface{} `yaml:"config"`
}

type PublisherRegistry struct {
	cron       *cron.Cron
	publishers map[string]PublisherConfig
	dbPath     string
}

var db *sql.DB
var yamlConfig *YAMLConfig

func main() {
	logger.Println("========== Starting GoBox ==========")
	loadYAMLConfig()
	initDatabase()
	defer db.Close()

	logger.Println("Starting publisher registry and server...")
	registry := NewPublisherRegistry(yamlConfig.Publishers, yamlConfig.Database.Connection)
	registry.Start()
	defer registry.Stop()

	// Gin server frontend runs separately
	go startServer()

	// Block thread to keep main alive
	select {}
}

func loadYAMLConfig() {
	logger.Println("[CONFIG] Loading config.yaml...")

	data, err := os.ReadFile("config.yaml")
	if err != nil {
		logger.Fatalf("[CONFIG] ERROR: Failed to read config.yaml: %v\n", err)
	}

	yamlConfig = &YAMLConfig{}
	if err := yaml.Unmarshal(data, yamlConfig); err != nil {
		logger.Fatalf("[CONFIG] ERROR: Failed to parse config.yaml: %v\n", err)
	}

	logger.Printf("[CONFIG] Database Type: %s\n", yamlConfig.Database.Type)
	logger.Printf("[CONFIG] Server Port: %s\n", yamlConfig.Server.Port)
	logger.Printf("[CONFIG] Publishers found: %d\n", len(yamlConfig.Publishers))
	for name := range yamlConfig.Publishers {
		logger.Printf("[CONFIG] - Publisher: %s\n", name)
	}
	logger.Println("[CONFIG] Configuration loaded successfully")
}

func initDatabase() {
	logger.Println("[DATABASE] Opening connection...")
	unsafeDB, err := sql.Open(yamlConfig.Database.Type, yamlConfig.Database.Connection)
	if err != nil {
		logger.Fatalf("[DATABASE] ERROR: Failed to open database: %v\n", err)
	}

	if err := unsafeDB.Ping(); err != nil {
		logger.Fatalf("[DATABASE] ERROR: Failed to ping database: %v\n", err)
	}

	db = unsafeDB
	logger.Println("[DATABASE] Connected successfully")
	initDBTables()
}

func initDBTables() {
	logger.Println("[DATABASE] Initializing tables...")
	var createTableSQL string

	switch yamlConfig.Database.Type {
	case "sqlite3":
		createTableSQL = `
        CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
			level TEXT NOT NULL,
            time INTEGER NOT NULL,
            service TEXT NOT NULL,
            message TEXT NOT NULL
        );
        `
	case "mysql":
		createTableSQL = `
        CREATE TABLE IF NOT EXISTS events (
            id INT AUTO_INCREMENT PRIMARY KEY,
			level VARCHAR(10) NOT NULL,
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

func NewPublisherRegistry(publishers map[string]PublisherConfig, dbPath string) *PublisherRegistry {
	return &PublisherRegistry{
		cron:       cron.New(),
		publishers: publishers,
		dbPath:     dbPath,
	}
}

func (pr *PublisherRegistry) Start() {
	logger.Println("[REGISTRY] Starting publisher registry...")

	for name, pubConfig := range pr.publishers {
		logger.Printf("[REGISTRY] Registering publisher: %s (schedule: %s)\n", name, pubConfig.Schedule)

		publisherName := name
		publisherConfig := pubConfig

		_, err := pr.cron.AddFunc(publisherConfig.Schedule, func() {
			pr.runPublisher(publisherName, publisherConfig)
		})

		if err != nil {
			logger.Printf("[REGISTRY] ERROR: Failed to schedule publisher %s: %v\n", publisherName, err)
			continue
		}

		logger.Printf("[REGISTRY] Publisher %s scheduled successfully\n", publisherName)
	}

	pr.cron.Start()
	logger.Println("[REGISTRY] Publisher registry started")
}

func (pr *PublisherRegistry) runPublisher(name string, pubConfig PublisherConfig) {
	logger.Printf("[REGISTRY] Running publisher: %s\n", name)

	configJSON, err := json.Marshal(pubConfig.Config)
	if err != nil {
		logger.Printf("[REGISTRY] ERROR: Failed to marshal config for %s: %v\n", name, err)
		return
	}

	cmd := exec.Command(pubConfig.Executable)
	cmd.Stdin = bytes.NewReader(configJSON)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Printf("[REGISTRY] ERROR: Publisher %s failed: %v\n", name, err)
		return
	}

	logger.Printf("[REGISTRY] Publisher %s completed successfully\n", name)
}

func (pr *PublisherRegistry) Stop() {
	logger.Println("[REGISTRY] Stopping publisher registry...")
	pr.cron.Stop()
	logger.Println("[REGISTRY] Publisher registry stopped")
}

// Frontend

func startServer() {
	r := gin.Default()
	r.Static("/static", "./static")
	r.GET("/read", readEvents)

	logger.Printf("[SERVER] Starting on %s\n", yamlConfig.Server.Port)
	r.Run(yamlConfig.Server.Port)
}

type Event struct {
	Id      int
	Level   string
	Time    int
	Service string
	Message string
}

type PageData struct {
	Entries    []Event
	Limit      string
	Offset     string
	Page       int
	NextOffset int
	PrevOffset int
	Level      string
}

func readEvents(c *gin.Context) {
	limit := c.DefaultQuery("limit", "50")
	offset := c.DefaultQuery("offset", "0")
	level := c.DefaultQuery("level", "info")

	logger.Printf("[QUERY] Reading events - Limit: %s, Offset: %s, Level: %s\n", limit, offset, level)

	var whereClause string
	switch level {
	case "crit":
		whereClause = " WHERE level = 'CRIT'"
	case "warn":
		whereClause = " WHERE level IN ('WARN', 'CRIT')"
	default:
		whereClause = ""
	}

	query := "SELECT id, level, time, service, message FROM events" + whereClause + " ORDER BY time DESC LIMIT ? OFFSET ?"
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
		if err := rows.Scan(&e.Id, &e.Level, &e.Time, &e.Service, &e.Message); err != nil {
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

	data.Level = level

	tmpl := template.New("template.html").Funcs(map[string]any{
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
