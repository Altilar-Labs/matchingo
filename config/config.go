package config

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/erain9/matchingo/pkg/db/queue"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server struct {
		GRPCAddr  string `yaml:"grpc_addr"`
		HTTPAddr  string `yaml:"http_addr"`
		LogLevel  string `yaml:"log_level"`
		LogFormat string `yaml:"log_format"`
	} `yaml:"server"`

	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`

	Kafka struct {
		BrokerAddr string `yaml:"broker_addr"`
		Topic      string `yaml:"topic"`
	} `yaml:"kafka"`
}

// Default configuration values
var (
	configFile = flag.String("config", "", "Path to config file (YAML)")
	grpcPort   = flag.Int("grpc_port", 50051, "The gRPC server port")
	httpPort   = flag.Int("http_port", 8080, "The HTTP server port")
	logLevel   = flag.String("log_level", "info", "Log level: debug, info, warn, error")
	logFormat  = flag.String("log_format", "pretty", "Log format: json, pretty")
)

// LoadConfig loads the configuration from command line flags and optionally from a config file
func LoadConfig() (*Config, error) {
	// Parse command line flags
	flag.Parse()

	// Create default configuration
	config := &Config{}
	config.Server.GRPCAddr = fmt.Sprintf(":%d", *grpcPort)
	config.Server.HTTPAddr = fmt.Sprintf(":%d", *httpPort)
	config.Server.LogLevel = *logLevel
	config.Server.LogFormat = *logFormat
	config.Redis.Addr = "localhost:6379"
	config.Kafka.BrokerAddr = "localhost:9092"
	config.Kafka.Topic = "test-msg-queue"

	// Load configuration from file if specified
	if *configFile != "" {
		yamlFile, err := os.ReadFile(*configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		// Parse YAML configuration
		if err := yaml.Unmarshal(yamlFile, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}

		// Override Kafka and Redis configuration in package variables
		queue.SetBrokerList(config.Kafka.BrokerAddr)
		queue.SetTopic(config.Kafka.Topic)

		// Log loaded configuration
		log.Printf("Loaded configuration from %s", *configFile)
	}

	return config, nil
}
