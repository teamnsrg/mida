package main

import (
	"github.com/spf13/viper"
)

// initViperConfig
func initViperConfig() {
	// Initialize the hardcoded defaults
	setDefaults()

	// We will read environment variables with the "MIDA" prefix
	viper.SetEnvPrefix("MIDA")
	viper.AutomaticEnv()
}

// Hardcoded default configuration values
func setDefaults() {
	// MIDA-Wide Configuration Defaults
	viper.SetDefault("crawlers", 1)
	viper.SetDefault("storers", 1)
	viper.SetDefault("prom-port", 8001)
	viper.SetDefault("monitor", false)
	viper.SetDefault("log-level", 2)
	viper.SetDefault("task-file", "examples/example_task.json")

	viper.SetDefault("amqp-user", "")
	viper.SetDefault("amqp-pass", "")
	viper.SetDefault("amqp-uri", "amqp://localhost:5672")
	viper.SetDefault("amqp-task-queue", "mida-tasks")
}
