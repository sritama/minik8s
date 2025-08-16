package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration
type Config struct {
	// Server configuration
	Port int
	Host string

	// Store configuration
	StoreType      string
	EtcdEndpoints  []string
	StorePrefix    string
	EnableFallback bool

	// Logging configuration
	LogLevel string
	LogJSON  bool

	// Development configuration
	DevMode bool
}

// Load loads configuration from environment variables
func Load() *Config {
	config := &Config{
		// Server defaults
		Port: getEnvAsInt("MINIK8S_PORT", 8080),
		Host: getEnv("MINIK8S_HOST", "0.0.0.0"),

		// Store defaults
		StoreType:      getEnv("MINIK8S_STORE_TYPE", "memory"),
		EtcdEndpoints:  strings.Split(getEnv("MINIK8S_ETCD_ENDPOINTS", "localhost:2379"), ","),
		StorePrefix:    getEnv("MINIK8S_STORE_PREFIX", "/minik8s"),
		EnableFallback: getEnvAsBool("MINIK8S_ENABLE_FALLBACK", true),

		// Logging defaults
		LogLevel: getEnv("MINIK8S_LOG_LEVEL", "info"),
		LogJSON:  getEnvAsBool("MINIK8S_LOG_JSON", false),

		// Development defaults
		DevMode: getEnvAsBool("MINIK8S_DEV_MODE", false),
	}

	return config
}

// LoadFromFile loads configuration from a file (future enhancement)
func LoadFromFile(filename string) (*Config, error) {
	// TODO: Implement file-based configuration
	// For now, just return environment-based config
	return Load(), nil
}

// IsEtcdStore returns true if the store type is etcd
func (c *Config) IsEtcdStore() bool {
	return c.StoreType == "etcd"
}

// IsMemoryStore returns true if the store type is memory
func (c *Config) IsMemoryStore() bool {
	return c.StoreType == "memory"
}

// GetStoreConfig returns the store configuration
func (c *Config) GetStoreConfig() map[string]interface{} {
	return map[string]interface{}{
		"type":      c.StoreType,
		"endpoints": c.EtcdEndpoints,
		"prefix":    c.StorePrefix,
		"fallback":  c.EnableFallback,
	}
}

// GetServerConfig returns the server configuration
func (c *Config) GetServerConfig() map[string]interface{} {
	return map[string]interface{}{
		"port": c.Port,
		"host": c.Host,
	}
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
