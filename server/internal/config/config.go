package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

type Config struct {
	App       AppConfig
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Auth      AuthConfig
	Storage   StorageConfig
	SMS       SMSConfig
	Email     EmailConfig
	RateLimit RateLimitConfig
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
	TrustedProxies    []string
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

type AuthConfig struct {
	JWTSecret      string
	AccessTokenTTL time.Duration
}

type StorageConfig struct {
	Enabled   bool
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type SMSConfig struct {
	Enabled      bool
	APIKey       string
	APISecret    string
	SignName     string
	TemplateCode string
}

type EmailConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	FromName string
}

type RateLimitConfig struct {
	Enabled       bool
	GlobalRPM     int
	AuthRPM       int
	VerifyCodeRPM int
	AdminRPM      int
}

func (c DatabaseConfig) DSN() string {
	cfg := mysql.NewConfig()
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	cfg.DBName = c.Name
	if c.Params != "" {
		params, _ := url.ParseQuery(c.Params)
		if len(params) > 0 && cfg.Params == nil {
			cfg.Params = make(map[string]string, len(params))
		}
		for k, v := range params {
			if len(v) > 0 {
				cfg.Params[k] = v[0]
			}
		}
	}
	return cfg.FormatDSN()
}

func Load() (Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("./server")
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return Config{}, fmt.Errorf("failed to read config: %w", err)
		}
	}

	// AutomaticEnv must be called after ReadInConfig so that environment
	// variables take precedence over .env file values.
	v.AutomaticEnv()

	cfg := Config{
		App: AppConfig{
			Name: v.GetString("APP_NAME"),
			Env:  v.GetString("APP_ENV"),
		},
		Server: ServerConfig{
			Host:              v.GetString("SERVER_HOST"),
			Port:              v.GetInt("SERVER_PORT"),
			ReadHeaderTimeout: v.GetDuration("SERVER_READ_HEADER_TIMEOUT"),
			ShutdownTimeout:   v.GetDuration("SERVER_SHUTDOWN_TIMEOUT"),
			TrustedProxies:    splitCSV(v.GetString("SERVER_TRUSTED_PROXIES")),
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
		Auth: AuthConfig{
			JWTSecret:      v.GetString("AUTH_JWT_SECRET"),
			AccessTokenTTL: v.GetDuration("AUTH_ACCESS_TOKEN_TTL"),
		},
		Storage: StorageConfig{
			Enabled:   v.GetBool("STORAGE_ENABLED"),
			Endpoint:  v.GetString("STORAGE_ENDPOINT"),
			AccessKey: v.GetString("STORAGE_ACCESS_KEY"),
			SecretKey: v.GetString("STORAGE_SECRET_KEY"),
			Bucket:    v.GetString("STORAGE_BUCKET"),
			UseSSL:    v.GetBool("STORAGE_USE_SSL"),
		},
		SMS: SMSConfig{
			Enabled:      v.GetBool("SMS_ENABLED"),
			APIKey:       v.GetString("SMS_API_KEY"),
			APISecret:    v.GetString("SMS_API_SECRET"),
			SignName:     v.GetString("SMS_SIGN_NAME"),
			TemplateCode: v.GetString("SMS_TEMPLATE_CODE"),
		},
		Email: EmailConfig{
			Enabled:  v.GetBool("EMAIL_ENABLED"),
			Host:     v.GetString("EMAIL_HOST"),
			Port:     v.GetInt("EMAIL_PORT"),
			Username: v.GetString("EMAIL_USERNAME"),
			Password: v.GetString("EMAIL_PASSWORD"),
			FromName: v.GetString("EMAIL_FROM_NAME"),
		},
		RateLimit: RateLimitConfig{
			Enabled:       v.GetBool("RATE_LIMIT_ENABLED"),
			GlobalRPM:     v.GetInt("RATE_LIMIT_GLOBAL_RPM"),
			AuthRPM:       v.GetInt("RATE_LIMIT_AUTH_RPM"),
			VerifyCodeRPM: v.GetInt("RATE_LIMIT_VERIFY_CODE_RPM"),
			AdminRPM:      v.GetInt("RATE_LIMIT_ADMIN_RPM"),
		},
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_NAME", "sdu-alumni-platform")
	v.SetDefault("APP_ENV", EnvDevelopment)
	v.SetDefault("SERVER_HOST", "0.0.0.0")
	v.SetDefault("SERVER_PORT", 8080)
	v.SetDefault("SERVER_READ_HEADER_TIMEOUT", 5*time.Second)
	v.SetDefault("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second)
	v.SetDefault("SERVER_TRUSTED_PROXIES", "")
	v.SetDefault("DB_ENABLED", false)
	v.SetDefault("DB_HOST", "127.0.0.1")
	v.SetDefault("DB_PORT", 3306)
	v.SetDefault("DB_USER", "sdu_alumni")
	v.SetDefault("DB_PASSWORD", "")
	v.SetDefault("DB_NAME", "sdu_alumni_db")
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
	v.SetDefault("AUTH_JWT_SECRET", "dev-only-change-me")
	v.SetDefault("AUTH_ACCESS_TOKEN_TTL", 24*time.Hour)
	v.SetDefault("STORAGE_ENABLED", false)
	v.SetDefault("STORAGE_ENDPOINT", "127.0.0.1:9000")
	v.SetDefault("STORAGE_ACCESS_KEY", "minioadmin")
	v.SetDefault("STORAGE_SECRET_KEY", "minioadmin123")
	v.SetDefault("STORAGE_BUCKET", "sdu-alumni-files")
	v.SetDefault("STORAGE_USE_SSL", false)
	v.SetDefault("SMS_ENABLED", false)
	v.SetDefault("SMS_API_KEY", "")
	v.SetDefault("SMS_API_SECRET", "")
	v.SetDefault("SMS_SIGN_NAME", "山东大学政管学院")
	v.SetDefault("SMS_TEMPLATE_CODE", "")
	v.SetDefault("EMAIL_ENABLED", false)
	v.SetDefault("EMAIL_HOST", "")
	v.SetDefault("EMAIL_PORT", 465)
	v.SetDefault("EMAIL_USERNAME", "")
	v.SetDefault("EMAIL_PASSWORD", "")
	v.SetDefault("EMAIL_FROM_NAME", "山东大学政管学院")
	v.SetDefault("RATE_LIMIT_ENABLED", false)
	v.SetDefault("RATE_LIMIT_GLOBAL_RPM", 120)
	v.SetDefault("RATE_LIMIT_AUTH_RPM", 10)
	v.SetDefault("RATE_LIMIT_VERIFY_CODE_RPM", 3)
	v.SetDefault("RATE_LIMIT_ADMIN_RPM", 30)
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
