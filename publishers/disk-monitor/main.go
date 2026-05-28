package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/kyle-t-r/gobox/publib"
	"github.com/shirou/gopsutil/v3/disk"
)

var logger = log.New(os.Stdout, "[DiskMonitor] ", log.LstdFlags)

type DiskMonitorConfig struct {
	DiskLabel    string  `json:"disk_label"`
	DiskWarn     float64 `json:"disk_warn"`
	DiskCritical float64 `json:"disk_critical"`
}

func main() {
	logger.Println("Starting disk-monitor publisher...")

	var config DiskMonitorConfig
	if err := publib.ConfigFromStdin(&config); err != nil {
		logger.Fatalf("ERROR: Failed to read config: %v\n", err)
	}

	logger.Printf("Config loaded: Disk \"%s\" - Warn: %.1f%%, Critical: %.1f%%\n", config.DiskLabel, config.DiskWarn, config.DiskCritical)

	db, err := publib.GetDB("./sqlite.db")
	if err != nil {
		logger.Fatalf("ERROR: Failed to connect to database: %v\n", err)
	}
	defer db.Close()

	logger.Printf("Checking disk \"%s\" usage...\n", config.DiskLabel)
	checkDisk(db, config)

	logger.Println("Checks complete")
}

func checkDisk(db *sql.DB, config DiskMonitorConfig) {
	diskUsage, err := disk.Usage(config.DiskLabel)
	if err != nil {
		logger.Printf("ERROR: Failed to get disk usage for %s: %v\n", config.DiskLabel, err)
		return
	}

	logger.Printf("[DISK \"%s\"] Current: %.2f%% (Warn: %.2f%%, Critical: %.2f%%)\n",
		config.DiskLabel, diskUsage.UsedPercent, config.DiskWarn, config.DiskCritical)

	var currentState string
	if diskUsage.UsedPercent > config.DiskCritical {
		currentState = "critical"
	} else if diskUsage.UsedPercent > config.DiskWarn {
		currentState = "warning"
	} else {
		currentState = "normal"
	}

	switch currentState {
	case "critical":
		logger.Printf("[DISK] CRITICAL: %s usage: %.2f%%\n", config.DiskLabel, diskUsage.UsedPercent)
		publib.InsertEvent(db, publib.Event{
			Time:    publib.GetCurrentTime(),
			Level:   "CRIT",
			Service: "disk-monitor",
			Message: fmt.Sprintf("Disk %s usage critical: %.2f%%", config.DiskLabel, diskUsage.UsedPercent),
		})
	case "warning":
		logger.Printf("[DISK] WARNING: %s usage: %.2f%%\n", config.DiskLabel, diskUsage.UsedPercent)
		publib.InsertEvent(db, publib.Event{
			Time:    publib.GetCurrentTime(),
			Level:   "WARN",
			Service: "disk-monitor",
			Message: fmt.Sprintf("Disk %s usage warning: %.2f%%", config.DiskLabel, diskUsage.UsedPercent),
		})
	default:
		logger.Printf("[DISK] INFO: %s usage: %.2f%%\n", config.DiskLabel, diskUsage.UsedPercent)
		publib.InsertEvent(db, publib.Event{
			Time:    publib.GetCurrentTime(),
			Level:   "INFO",
			Service: "disk-monitor",
			Message: fmt.Sprintf("Disk %s usage: %.2f%%", config.DiskLabel, diskUsage.UsedPercent),
		})
	}
}
