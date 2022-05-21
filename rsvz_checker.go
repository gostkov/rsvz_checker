package main

import (
	"github.com/spf13/viper"
	"log"
)

type Config struct {
	URLs            string `mapstructure:"URLs"`
	RefreshInterval string `mapstructure:"RefreshInterval"`
	ServerIP        string `mapstructure:"ServerIP"`
	ServerPort      string `mapstructure:"ServerPort"`
}

func LoadConfiguration(path string) (config Config) {
	viper.AddConfigPath(path)
	viper.SetConfigName("rsvz_checker")
	viper.SetConfigType("yaml")
}

func main() {
	log.Println("Runnning")

}
