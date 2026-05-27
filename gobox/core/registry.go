package core

import (
	"bytes"
	"encoding/json"
	"log"
	"os"

	"os/exec"

	"github.com/robfig/cron/v3"
)

/**
 * Publisher registry implementation for GoBox
 * Manages scheduled publishers that execute external commands based on YAML configuration.
 */

var (
	logger = log.New(os.Stdout, "[EDA] ", log.LstdFlags)
)

type PublisherConfig struct {
	Schedule   string         `yaml:"schedule"`
	Executable string         `yaml:"executable"`
	Config     map[string]any `yaml:"config"`
}

type PublisherRegistry struct {
	cron       *cron.Cron
	publishers map[string]PublisherConfig
}

func NewPublisherRegistry(publishers map[string]PublisherConfig) *PublisherRegistry {
	return &PublisherRegistry{
		cron:       cron.New(),
		publishers: publishers,
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

	cmd := exec.Command("bash", "-c", pubConfig.Executable)
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
