package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	JWT      JWTConfig      `yaml:"jwt"`
	CORS     CORSConfig     `yaml:"cors"`
	LogLevel string         `yaml:"log_level"`
}

type ServerConfig struct {
	Port            string `yaml:"port"`
	Host            string `yaml:"host"`
	ReadTimeout     int    `yaml:"read_timeout"`
	WriteTimeout    int    `yaml:"write_timeout"`
	IdleTimeout     int    `yaml:"idle_timeout"`
	ShutdownTimeout int    `yaml:"shutdown_timeout"`
}

type DatabaseConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Name            string `yaml:"name"`
	SSLMode         string `yaml:"ssl_mode"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime int    `yaml:"conn_max_idle_time"`
}

type RedisConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
	DialTimeout  int    `yaml:"dial_timeout"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
	IdleTimeout  int    `yaml:"idle_timeout"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
	TTL    int    `yaml:"ttl"`
}

type CORSConfig struct {
	AllowedOrigins string `yaml:"allowed_origins"`
}

func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port:            getEnv("PORT", "8084"),
			Host:            getEnv("HOST", "0.0.0.0"),
			ReadTimeout:     getEnvAsInt("SERVER_READ_TIMEOUT", 10),
			WriteTimeout:    getEnvAsInt("SERVER_WRITE_TIMEOUT", 10),
			IdleTimeout:     getEnvAsInt("SERVER_IDLE_TIMEOUT", 120),
			ShutdownTimeout: getEnvAsInt("SERVER_SHUTDOWN_TIMEOUT", 30),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "postgres"),
			Name:            getEnv("DB_NAME", "workspace_service_db"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvAsInt("DB_CONN_MAX_LIFETIME", 300),
			ConnMaxIdleTime: getEnvAsInt("DB_CONN_MAX_IDLE_TIME", 60),
		},
		Redis: RedisConfig{
			Host:         getEnv("REDIS_HOST", "localhost"),
			Port:         getEnvAsInt("REDIS_PORT", 6379),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvAsInt("REDIS_DB", 0),
			PoolSize:     getEnvAsInt("REDIS_POOL_SIZE", 10),
			MinIdleConns: getEnvAsInt("REDIS_MIN_IDLE_CONNS", 2),
			DialTimeout:  getEnvAsInt("REDIS_DIAL_TIMEOUT", 5),
			ReadTimeout:  getEnvAsInt("REDIS_READ_TIMEOUT", 3),
			WriteTimeout: getEnvAsInt("REDIS_WRITE_TIMEOUT", 3),
			IdleTimeout:  getEnvAsInt("REDIS_IDLE_TIMEOUT", 300),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "your-secret-key"),
			TTL:    getEnvAsInt("JWT_TTL", 3600),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "*"),
		},
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func (c *CORSConfig) GetAllowedOrigins() []string {
	if c.AllowedOrigins == "*" {
		return []string{"*"}
	}
	return strings.Split(c.AllowedOrigins, ",")
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

// GetRedisAddr returns the Redis address string
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}