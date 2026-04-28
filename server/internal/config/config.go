package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type ServerConfig struct {
	Host              string
	Port              int
	ReadHeaderTimeout time.Duration
	ShutdownTimeout   time.Duration
}

func (c ServerConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

type DatabaseConfig struct {
	Enabled         bool
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	Params          string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Enabled      bool
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?%s",
		c.User,
		c.Password,
		net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		c.Name,
		c.Params,
	)
}

func Load() Config {
	v := viper.New()
	setDefaults(v)

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("./server")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			panic(fmt.Errorf("failed to read config: %w", err))
		}
	}

	return Config{
		App: AppConfig{
			Name: v.GetString("APP_NAME"),
			Env:  v.GetString("APP_ENV"),
		},
		Server: ServerConfig{
			Host:              v.GetString("SERVER_HOST"),
			Port:              v.GetInt("SERVER_PORT"),
			ReadHeaderTimeout: v.GetDuration("SERVER_READ_HEADER_TIMEOUT"),
			ShutdownTimeout:   v.GetDuration("SERVER_SHUTDOWN_TIMEOUT"),
		},
		Database: DatabaseConfig{
			Enabled:         v.GetBool("DB_ENABLED"),
			Host:            v.GetString("DB_HOST"),
			Port:            v.GetInt("DB_PORT"),
			User:            v.GetString("DB_USER"),
			Password:        v.GetString("DB_PASSWORD"),
			Name:            v.GetString("DB_NAME"),
			Params:          v.GetString("DB_PARAMS"),
			MaxOpenConns:    v.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    v.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: v.GetDuration("DB_CONN_MAX_LIFETIME"),
		},
		Redis: RedisConfig{
			Enabled:      v.GetBool("REDIS_ENABLED"),
			Addr:         v.GetString("REDIS_ADDR"),
			Password:     v.GetString("REDIS_PASSWORD"),
			DB:           v.GetInt("REDIS_DB"),
			PoolSize:     v.GetInt("REDIS_POOL_SIZE"),
			MinIdleConns: v.GetInt("REDIS_MIN_IDLE_CONNS"),
			DialTimeout:  v.GetDuration("REDIS_DIAL_TIMEOUT"),
			ReadTimeout:  v.GetDuration("REDIS_READ_TIMEOUT"),
			WriteTimeout: v.GetDuration("REDIS_WRITE_TIMEOUT"),
		},
	}
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_NAME", "sdu-alumni-platform")
	v.SetDefault("APP_ENV", EnvDevelopment)
	v.SetDefault("SERVER_HOST", "0.0.0.0")
	v.SetDefault("SERVER_PORT", 8080)
	v.SetDefault("SERVER_READ_HEADER_TIMEOUT", 5*time.Second)
	v.SetDefault("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second)
	v.SetDefault("DB_ENABLED", false)
	v.SetDefault("DB_HOST", "127.0.0.1")
	v.SetDefault("DB_PORT", 3306)
	v.SetDefault("DB_USER", "sdu_alumni")
	v.SetDefault("DB_PASSWORD", "")
	v.SetDefault("DB_NAME", "sdu_alumni")
	v.SetDefault("DB_PARAMS", "charset=utf8mb4&parseTime=true&loc=Local")
	v.SetDefault("DB_MAX_OPEN_CONNS", 25)
	v.SetDefault("DB_MAX_IDLE_CONNS", 5)
	v.SetDefault("DB_CONN_MAX_LIFETIME", time.Hour)
	v.SetDefault("REDIS_ENABLED", false)
	v.SetDefault("REDIS_ADDR", "127.0.0.1:6379")
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("REDIS_POOL_SIZE", 10)
	v.SetDefault("REDIS_MIN_IDLE_CONNS", 2)
	v.SetDefault("REDIS_DIAL_TIMEOUT", 5*time.Second)
	v.SetDefault("REDIS_READ_TIMEOUT", 3*time.Second)
	v.SetDefault("REDIS_WRITE_TIMEOUT", 3*time.Second)
}
