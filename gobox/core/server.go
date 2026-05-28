package core

import (
	"database/sql"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kyle-t-r/gobox/publib"
)

/**
 * Core server implementation for GoBox
 * Handles HTTP requests, serves the frontend, and interacts with the database to fetch event logs.
 */

type Server struct {
	db   *sql.DB
	port string
}

func NewServer(db *sql.DB, port string) *Server {
	return &Server{
		db:   db,
		port: port,
	}
}

func (s *Server) Start() {
	r := gin.Default()
	r.Static("/static", "./static")
	r.GET("/read", func(c *gin.Context) {
		readEvents(c, s.db)
	})
	r.POST("/new", func(c *gin.Context) {
		newEvent(c, s.db)
	})
	r.GET("/socket", handleWebSocket)

	logger.Printf("[SERVER] Starting on %s\n", s.port)
	r.Run(s.port)
}

func handleWebSocket(c *gin.Context) {
	// TODO
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
	Service    string
	Services   []string
}

func readEvents(c *gin.Context, db *sql.DB) {
	limit := c.DefaultQuery("limit", "50")
	offset := c.DefaultQuery("offset", "0")
	level := c.DefaultQuery("level", "info")
	service := c.DefaultQuery("service", "")

	logger.Printf("[QUERY] Reading events - Limit: %s, Offset: %s, Level: %s, Service: %s\n", limit, offset, level, service)
	where, hasService := whereClause(level, service)
	query := "SELECT id, level, time, service, message FROM events" + where + " ORDER BY time DESC LIMIT ? OFFSET ?"
	logger.Printf("[QUERY] Executing query: %s\n", query)

	var rows *sql.Rows
	var err error
	if hasService {
		rows, err = db.Query(query, service, limit, offset)
	} else {
		rows, err = db.Query(query, limit, offset)
	}
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
		Level:      level,
		Service:    service,
		Services:   getAllServices(db),
	}

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
	if err := tmpl.Execute(c.Writer, data); err != nil {
		logger.Printf("[TEMPLATE] ERROR: Failed to execute template: %v\n", err)
		c.String(500, "Template error")
		return
	}
}

func whereClause(level string, service string) (string, bool) {
	hasService := false
	var clauses []string

	if level != "" {
		switch level {
		case "crit":
			clauses = append(clauses, "level = 'CRIT'")
		case "warn":
			clauses = append(clauses, "level IN ('WARN', 'CRIT')")
		default:
			clauses = append(clauses, "level IN ('INFO', 'WARN', 'CRIT')")
		}
	}

	if service != "" {
		hasService = true
		clauses = append(clauses, "service = ?")
	}

	if len(clauses) == 0 {
		return "", false
	}

	return " WHERE " + strings.Join(clauses, " AND "), hasService
}

func getAllServices(db *sql.DB) []string {
	rows, err := db.Query("SELECT DISTINCT service FROM events")
	if err != nil {
		logger.Printf("[QUERY] ERROR: Failed to fetch services: %v\n", err)
		return []string{}
	}
	defer rows.Close()

	var services []string = []string{}
	for rows.Next() {
		var service string
		if err := rows.Scan(&service); err != nil {
			logger.Printf("[QUERY] ERROR: Failed to scan service: %v\n", err)
			continue
		}
		services = append(services, service)
	}

	return services
}

func newEvent(c *gin.Context, db *sql.DB) {
	var event publib.Event
	if err := c.BindJSON(&event); err != nil {
		logger.Printf("[SERVER] ERROR: Failed to bind JSON: %v\n", err)
		c.String(400, "Invalid request")
		return
	}

	if err := publib.InsertEvent(db, event); err != nil {
		c.String(500, "Database error")
		return
	}

	c.String(200, "Event created successfully")
}
