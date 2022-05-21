package main

import (
	"github.com/spf13/viper"
)

type Config struct {
	RefreshInterval int    `mapstructure:"REFRESH_INTERVAL"`
	BindPort        string `mapstructure:"SERVER_PORT"`
	BindIP          string `mapstructure:"SERVER_IP"`
	URLs            string `mapstructure:"URLS"`
}

func LoadConfiguration(path string) (config Config, err error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("SERVER_IP", "127.0.0.1")
	v.SetDefault("SERVER_PORT", "8080")
	v.SetDefault("REFRESH_INTERVAL", 1)
	v.AddConfigPath(path)
	v.SetConfigName("rsvz_checker")
	v.SetConfigType("env")
	_ = v.ReadInConfig()
	err = v.Unmarshal(&config)
	return config, err
}
