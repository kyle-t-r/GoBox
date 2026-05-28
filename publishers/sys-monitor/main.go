package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/kyle-t-r/gobox/publib"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

var logger = log.New(os.Stdout, "[SysMonitor] ", log.LstdFlags)

type SysMonitorConfig struct {
	CPUWarn     float64 `json:"cpu_warn"`
	CPUCritical float64 `json:"cpu_critical"`
	MemWarn     float64 `json:"mem_warn"`
	MemCritical float64 `json:"mem_critical"`
}

func main() {
	logger.Println("Starting sys-monitor publisher...")

	var config SysMonitorConfig
	if err := publib.ConfigFromStdin(&config); err != nil {
		logger.Fatalf("ERROR: Failed to read config: %v\n", err)
	}

	logger.Printf("Config loaded: CPU Warn: %.1f%%, CPU Critical: %.1f%%, Memory Warn: %.1f%%, Memory Critical: %.1f%%\n",
		config.CPUWarn, config.CPUCritical, config.MemWarn, config.MemCritical)

	db, err := publib.GetDB("./sqlite.db")
	if err != nil {
		logger.Fatalf("ERROR: Failed to connect to database: %v\n", err)
	}
	defer db.Close()

	logger.Println("Running system checks...")
	checkCPU(db, config)
	checkMemory(db, config)

	logger.Println("Checks complete")
}

func checkCPU(db *sql.DB, config SysMonitorConfig) {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		logger.Printf("ERROR: Failed to get CPU usage: %v\n", err)
		return
	}

	logger.Printf("[CPU] Current: %.2f%% (Threshold: %.2f%%)\n", cpuPercent[0], config.CPUWarn)

	var currentState string
	if cpuPercent[0] > config.CPUCritical {
		currentState = "critical"
	} else if cpuPercent[0] > config.CPUWarn {
		currentState = "warning"
	} else {
		currentState = "normal"
	}

	switch currentState {
	case "critical":
		logger.Printf("[CPU] CRITICAL: Threshold exceeded! Usage: %.2f%%\n", cpuPercent[0])
		publib.InsertEvent(db, publib.Event{
			Time:    publib.GetCurrentTime(),
			Level:   "CRIT",
			Service: "sys-monitor",
			Message: fmt.Sprintf("CPU usage critical: %.2f%%", cpuPercent[0]),
		})
	case "warning":
		logger.Printf("[CPU] WARNING: Usage above threshold: %.2f%%\n", cpuPercent[0])
		publib.InsertEvent(db, publib.Event{
			Time:    publib.GetCurrentTime(),
			Level:   "WARN",
			Service: "sys-monitor",
			Message: fmt.Sprintf("CPU usage warning: %.2f%%", cpuPercent[0]),
		})
	default:
		logger.Printf("[CPU] INFO: Usage: %.2f%%\n", cpuPercent[0])
		publib.InsertEvent(db, publib.Event{
			Time:    publib.GetCurrentTime(),
			Level:   "INFO",
			Service: "sys-monitor",
			Message: fmt.Sprintf("CPU usage: %.2f%%", cpuPercent[0]),
		})
	}
}

func checkMemory(db *sql.DB, config SysMonitorConfig) {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		logger.Printf("ERROR: Failed to get memory usage: %v\n", err)
		return
	}

	logger.Printf("[MEMORY] Current: %.2f%%\n", memStats.UsedPercent)

	var currentState string
	if memStats.UsedPercent > config.MemCritical {
		currentState = "critical"
	} else if memStats.UsedPercent > config.MemWarn {
		currentState = "warning"
	} else {
		currentState = "normal"
	}

	switch currentState {
	case "critical":
		logger.Printf("[MEMORY] CRITICAL: Threshold exceeded! Usage: %.2f%%\n", memStats.UsedPercent)
		publib.InsertEvent(db, publib.Event{
			Time:    publib.GetCurrentTime(),
			Level:   "CRIT",
			Service: "sys-monitor",
			Message: fmt.Sprintf("Memory usage critical: %.2f%%", memStats.UsedPercent),
		})
	case "warning":
		logger.Printf("[MEMORY] WARNING: Usage above threshold: %.2f%%\n", memStats.UsedPercent)
		publib.InsertEvent(db, publib.Event{
			Time:    publib.GetCurrentTime(),
			Level:   "WARN",
			Service: "sys-monitor",
			Message: fmt.Sprintf("Memory usage warning: %.2f%%", memStats.UsedPercent),
		})
	default:
		logger.Printf("[MEMORY] INFO: Usage: %.2f%%\n", memStats.UsedPercent)
		publib.InsertEvent(db, publib.Event{
			Time:    publib.GetCurrentTime(),
			Level:   "INFO",
			Service: "sys-monitor",
			Message: fmt.Sprintf("Memory usage: %.2f%%", memStats.UsedPercent),
		})
	}
}
