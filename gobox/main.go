package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/kyle-t-r/gobox/core"
	_ "github.com/mattn/go-sqlite3"
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

type PublisherRegistry core.PublisherRegistry

type YAMLConfig struct {
	Database   DatabaseConfig                  `yaml:"database"`
	Server     ServerConfig                    `yaml:"server"`
	Publishers map[string]core.PublisherConfig `yaml:"publishers"`
}

type DatabaseConfig struct {
	Type       string `yaml:"type"`
	Connection string `yaml:"connection"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

var db *sql.DB
var yamlConfig *YAMLConfig

func main() {
	logger.Println("========== Starting GoBox ==========")
	loadYAMLConfig()
	initDatabase()
	defer db.Close()

	logger.Println("Starting publisher registry and server...")
	registry := core.NewPublisherRegistry(yamlConfig.Publishers)
	server := core.NewServer(db, yamlConfig.Server.Port)

	// Start server and registry in separate goroutines
	go server.Start()
	go registry.Start()
	defer registry.Stop()

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
