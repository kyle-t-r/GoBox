package listeners

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

/**
 * StatsListener - A listener based on the gopsutil that monitors system stats on a Cron schedule.
 */

var logger = log.New(os.Stdout, "[Listener] ", log.LstdFlags)

type Thresholds struct {
	CPUWarnPercent      float64
	CPUCriticalPercent  float64
	MemWarnPercent      float64
	MemCriticalPercent  float64
	DiskWarnPercent     float64
	DiskCriticalPercent float64
}

type StatsListener struct {
	thresholds Thresholds
	ginURL     string
	httpClient *http.Client
	diskStates map[string]string
	cpuState   string
	memState   string
}

func NewStatsListener(ginURL string, thresholds Thresholds) *StatsListener {
	logger.Printf("Creating new StatsListener for %s\n", ginURL)
	return &StatsListener{
		thresholds: thresholds,
		ginURL:     ginURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		diskStates: make(map[string]string),
		cpuState:   "normal",
		memState:   "normal",
	}
}

func (sl *StatsListener) Start() *cron.Cron {
	c := cron.New()
	c.AddFunc("@every 1m", sl.checkStats)
	c.Start()
	logger.Println("Stats listener cron job started (runs every 1 minute)")
	return c
}

func (sl *StatsListener) checkStats() {
	logger.Println("Checking system stats...")
	sl.checkCPU()
	sl.checkMemory()
	sl.checkDisk("/")
	sl.checkDisk("/mnt")
}

func (sl *StatsListener) checkCPU() {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		logger.Printf("ERROR: Failed to get CPU usage: %v\n", err)
		return
	}

	logger.Printf("[CPU] Current: %.2f%% (Threshold: %.2f%%)\n", cpuPercent[0], sl.thresholds.CPUWarnPercent)

	var currentState string
	if cpuPercent[0] > sl.thresholds.CPUCriticalPercent {
		currentState = "critical"
	} else if cpuPercent[0] > sl.thresholds.CPUWarnPercent {
		currentState = "warning"
	} else {
		currentState = "normal"
	}

	// Only log if state changed
	if currentState != sl.cpuState {
		previousState := sl.cpuState
		sl.cpuState = currentState
		if currentState == "critical" {
			logger.Printf("[CPU] ALERT CRITICAL: Threshold exceeded! Usage: %.2f%%\n", cpuPercent[0])
			sl.logEvent("sys-monitor", fmt.Sprintf("CPU usage critical: %.2f%%", cpuPercent[0]))
		} else if currentState == "warning" {
			logger.Printf("[CPU] ALERT WARNING: Usage above threshold: %.2f%%\n", cpuPercent[0])
			sl.logEvent("sys-monitor", fmt.Sprintf("CPU usage warning: %.2f%%", cpuPercent[0]))
		} else if previousState == "critical" || previousState == "warning" {
			logger.Printf("[CPU] RECOVERED: Usage back to normal: %.2f%%\n", cpuPercent[0])
			sl.logEvent("sys-monitor", fmt.Sprintf("CPU usage recovered: %.2f%%", cpuPercent[0]))
		}
	}
}

func (sl *StatsListener) checkMemory() {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		logger.Printf("ERROR: Failed to get memory usage: %v\n", err)
		return
	}

	logger.Printf("[MEMORY] Current: %.2f%%\n", memStats.UsedPercent)

	var currentState string
	if memStats.UsedPercent > sl.thresholds.MemCriticalPercent {
		currentState = "critical"
	} else if memStats.UsedPercent > sl.thresholds.MemWarnPercent {
		currentState = "warning"
	} else {
		currentState = "normal"
	}

	// Only log if state changed
	if currentState != sl.memState {
		previousState := sl.memState
		sl.memState = currentState
		if currentState == "critical" {
			logger.Printf("[MEMORY] ALERT CRITICAL: Threshold exceeded! Usage: %.2f%%\n", memStats.UsedPercent)
			sl.logEvent("sys-monitor", fmt.Sprintf("Memory usage critical: %.2f%%", memStats.UsedPercent))
		} else if currentState == "warning" {
			logger.Printf("[MEMORY] ALERT WARNING: Usage above threshold: %.2f%%\n", memStats.UsedPercent)
			sl.logEvent("sys-monitor", fmt.Sprintf("Memory usage warning: %.2f%%", memStats.UsedPercent))
		} else if previousState == "critical" || previousState == "warning" {
			logger.Printf("[MEMORY] RECOVERED: Usage back to normal: %.2f%%\n", memStats.UsedPercent)
			sl.logEvent("sys-monitor", fmt.Sprintf("Memory usage recovered: %.2f%%", memStats.UsedPercent))
		}
	}
}

func (sl *StatsListener) checkDisk(diskId string) {
	diskUsage, err := disk.Usage(diskId)
	if err != nil {
		logger.Printf("ERROR: Failed to get disk usage for %s: %v\n", diskId, err)
		return
	}

	logger.Printf("[DISK-%s] Current: %.2f%% (Warn: %.2f%%, Critical: %.2f%%)\n",
		diskId, diskUsage.UsedPercent, sl.thresholds.DiskWarnPercent, sl.thresholds.DiskCriticalPercent)

	var currentState string
	if diskUsage.UsedPercent > sl.thresholds.DiskCriticalPercent {
		currentState = "critical"
	} else if diskUsage.UsedPercent > sl.thresholds.DiskWarnPercent {
		currentState = "warning"
	} else {
		currentState = "normal"
	}

	previousState := sl.diskStates[diskId]

	// Only log if state changed
	if currentState != previousState {
		sl.diskStates[diskId] = currentState

		if currentState == "critical" {
			logger.Printf("[DISK] ALERT CRITICAL: %s usage: %.2f%%\n", diskId, diskUsage.UsedPercent)
			sl.logEvent("sys-monitor", fmt.Sprintf("Disk %s usage critical: %.2f%%", diskId, diskUsage.UsedPercent))
		} else if currentState == "warning" {
			logger.Printf("[DISK] ALERT WARNING: %s usage: %.2f%%\n", diskId, diskUsage.UsedPercent)
			sl.logEvent("sys-monitor", fmt.Sprintf("Disk %s usage warning: %.2f%%", diskId, diskUsage.UsedPercent))
		} else if previousState == "critical" || previousState == "warning" {
			// Only log recovery if there was a previous alert
			logger.Printf("[DISK] RECOVERED: %s usage back to normal: %.2f%%\n", diskId, diskUsage.UsedPercent)
			sl.logEvent("sys-monitor", fmt.Sprintf("Disk %s usage recovered: %.2f%%", diskId, diskUsage.UsedPercent))
		}
	}
}

func (sl *StatsListener) logEvent(service, message string) {
	params := url.Values{}
	params.Add("service", service)
	params.Add("message", message)

	url := sl.ginURL + "/new?" + params.Encode()

	resp, err := sl.httpClient.Get(url)
	if err != nil {
		logger.Printf("ERROR: Failed to call Gin service for %s: %v\n", service, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		logger.Printf("WARNING: Gin service returned %d for %s: %s\n", resp.StatusCode, service, string(body))
		return
	}

	logger.Printf("[LOGGED] %s: %s\n", service, message)
}
