package config

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	FolderID            string       `mapstructure:"folder_id"`
	AuthKeyFile         string       `mapstructure:"auth_key_file"`
	UseWorkloadIdentity bool         `mapstructure:"use_workload_identity"`
	Server              ServerConfig `mapstructure:"server"`
}

type ServerConfig struct {
	NetworkInterface string `mapstructure:"network_interface"`
	WebhookPort      int    `mapstructure:"webhook_port"`
	HealthPort       int    `mapstructure:"health_port"`
}

func LoadConfig() (*Config, error) {
	// Configure Viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Environment variable settings
	viper.AutomaticEnv()

	// Define CLI flags
	pflag.String("folder-id", "", "Yandex Cloud folder ID")
	pflag.String("auth-key-file", "", "Path to Yandex Cloud service account key file")
	pflag.Bool("use-workload-identity", false, "Use workload identity as auth method")
	pflag.String("network-interface", "0.0.0.0", "Network interface to bind webhook server too")
	pflag.Int("webhook-port", 8888, "Port for webhook server")
	pflag.Int("health-port", 8080, "Port for health check server")
	pflag.Parse()

	// Bind CLI flags to Viper
	if err := viper.BindPFlag("folder_id", pflag.Lookup("folder-id")); err != nil {
		return nil, fmt.Errorf("error binding folder-id flag: %v", err)
	}
	if err := viper.BindPFlag("auth_key_file", pflag.Lookup("auth-key-file")); err != nil {
		return nil, fmt.Errorf("error binding auth-key-file flag: %v", err)
	}
	if err := viper.BindPFlag("use_workload_identity", pflag.Lookup("use-workload-identity")); err != nil {
		return nil, fmt.Errorf("error binding use-workload-identity flag: %v", err)
	}
	if err := viper.BindPFlag("server.webhook_port", pflag.Lookup("webhook-port")); err != nil {
		return nil, fmt.Errorf("error binding webhook-port flag: %v", err)
	}
	if err := viper.BindPFlag("server.health_port", pflag.Lookup("health-port")); err != nil {
		return nil, fmt.Errorf("error binding health-port flag: %v", err)
	}
	if err := viper.BindPFlag("server.network_interface", pflag.Lookup("network-interface")); err != nil {
		return nil, fmt.Errorf("error binding network-interface flag: %v", err)
	}

	// Set default values
	viper.SetDefault("server.webhook_port", 8888)
	viper.SetDefault("server.health_port", 8080)
	viper.SetDefault("server.network_interface", "0.0.0.0")

	// Read configuration file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %v", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %v", err)
	}

	// Validate required fields
	if config.FolderID == "" {
		return nil, fmt.Errorf("folder-id configuration is required")
	}

	authKeyFileUsed := config.AuthKeyFile != ""

	if !authKeyFileUsed && !config.UseWorkloadIdentity {
		return nil, fmt.Errorf("auth-key-file or use-workload-identity configuration is required")
	}

	if authKeyFileUsed && config.UseWorkloadIdentity {
		return nil, fmt.Errorf("auth-key-file and use-workload-identity cant be used together")
	}

	return &config, nil
}
