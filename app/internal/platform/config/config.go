// Package config loads layered configuration for LLMHub services.
//
// Sources (later sources override earlier ones):
//  1. defaults in code
//  2. configs/app.yaml
//  3. configs/app.<env>.yaml
//  4. configs/<service>.yaml (optional service-specific overrides)
//  5. environment variables prefixed with LLMHUB_
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the root configuration shared across all services.
// Each service only reads the subset it cares about; unknown fields are ignored.
type Config struct {
	Env     string        `mapstructure:"env"`     // dev / staging / prod
	Service string        `mapstructure:"service"` // gateway / scheduler / ...
	HTTP    HTTPConfig    `mapstructure:"http"`
	GRPC    GRPCConfig    `mapstructure:"grpc"`
	DB      DBConfig      `mapstructure:"db"`
	Redis   RedisConfig   `mapstructure:"redis"`
	NATS    NATSConfig    `mapstructure:"nats"`
	Vault   VaultConfig   `mapstructure:"vault"`
	Tracing TracingConfig `mapstructure:"tracing"`
	Log     LogConfig     `mapstructure:"log"`
}

type HTTPConfig struct {
	Addr            string `mapstructure:"addr"`
	ReadTimeoutSec  int    `mapstructure:"read_timeout_sec"`
	WriteTimeoutSec int    `mapstructure:"write_timeout_sec"`
}

type GRPCConfig struct {
	Addr string `mapstructure:"addr"`
	// mTLS: Vault-issued certs in production
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	CAFile   string `mapstructure:"ca_file"`
}

type DBConfig struct {
	DSN             string `mapstructure:"dsn"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifeMins int    `mapstructure:"conn_max_life_mins"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type NATSConfig struct {
	URL string `mapstructure:"url"`
}

type VaultConfig struct {
	Addr      string `mapstructure:"addr"`
	Token     string `mapstructure:"token"`
	MountPath string `mapstructure:"mount_path"`
}

type TracingConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	OTLPEndpoint string  `mapstructure:"otlp_endpoint"`
	SampleRatio  float64 `mapstructure:"sample_ratio"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug / info / warn / error
	Format string `mapstructure:"format"` // json / text
}

// Load reads configuration for a given service.
// The service parameter is used to read optional service-specific overrides
// and to tag logs/metrics with the service name.
func Load(service string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath("/etc/llmhub")

	// Defaults
	setDefaults(v, service)

	// Base
	v.SetConfigName("app")
	if err := v.ReadInConfig(); err != nil {
		if _, notFound := err.(viper.ConfigFileNotFoundError); !notFound {
			return nil, fmt.Errorf("read app config: %w", err)
		}
	}

	// Env overlay
	env := v.GetString("env")
	if env != "" {
		v.SetConfigName("app." + env)
		if err := v.MergeInConfig(); err != nil {
			if _, notFound := err.(viper.ConfigFileNotFoundError); !notFound {
				return nil, fmt.Errorf("read env config: %w", err)
			}
		}
	}

	// Service-specific overlay
	v.SetConfigName(service)
	if err := v.MergeInConfig(); err != nil {
		if _, notFound := err.(viper.ConfigFileNotFoundError); !notFound {
			return nil, fmt.Errorf("read service config: %w", err)
		}
	}

	// ENV overrides: LLMHUB_HTTP_ADDR=:8080 etc.
	v.SetEnvPrefix("LLMHUB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	cfg.Service = service
	return &cfg, nil
}

func setDefaults(v *viper.Viper, service string) {
	v.SetDefault("env", "dev")
	v.SetDefault("service", service)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	// Default ports per service
	switch service {
	case "gateway":
		v.SetDefault("http.addr", ":8080")
	case "scheduler":
		v.SetDefault("grpc.addr", ":9001")
	case "account":
		v.SetDefault("http.addr", ":8081")
		v.SetDefault("grpc.addr", ":9002")
	case "billing":
		v.SetDefault("grpc.addr", ":9003")
	}

	v.SetDefault("http.read_timeout_sec", 30)
	v.SetDefault("http.write_timeout_sec", 120)
	v.SetDefault("db.max_open_conns", 20)
	v.SetDefault("db.max_idle_conns", 5)
	v.SetDefault("db.conn_max_life_mins", 30)
}
