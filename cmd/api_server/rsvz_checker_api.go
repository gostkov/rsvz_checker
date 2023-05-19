package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"net/http"
	"os"
	rsvz_checker "rsvz_checker/pkg"
	"strconv"
)

type AppConfig struct {
	RefreshInterval int      `mapstructure:"REFRESH_INTERVAL"`
	BindPort        string   `mapstructure:"SERVER_PORT"`
	BindIP          string   `mapstructure:"SERVER_IP"`
	URLs            []string `mapstructure:"URLS"`
	log             *zap.SugaredLogger
	registry        *rsvz_checker.Registry
}

func (h *AppConfig) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.log.Info("TEST")
	h.log.Infof("incoming request: %s", req.URL.RawQuery)
	if req.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = fmt.Fprintf(w, "Not supported method!")
		return
	}
	field := req.URL.Query().Get("field")
	parsedResult, err := rsvz_checker.IncomingRFPhoneProcessing(req.URL.Query().Get("num"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, err.Error())
		return
	}
	parsedResult.Operator, parsedResult.Region = h.registry.SearchCodeByPrefixAndPhone(parsedResult.Code, parsedResult.Phone)
	h.log.Infof("Parced: code: %d, phone: %d, operator: %s, region: %s",
		parsedResult.Code, parsedResult.Phone, parsedResult.Operator, parsedResult.Region)

	if field == "" {
		jsonResp, _ := json.Marshal(parsedResult)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, err = w.Write(jsonResp)
	} else {
		switch field {
		case "code":
			w.Write([]byte(strconv.Itoa(parsedResult.Code)))
		case "full_name":
			w.Write([]byte(parsedResult.FullNum))
		case "operator":
			w.Write([]byte(parsedResult.Operator))
		case "region":
			w.Write([]byte(parsedResult.Region))
		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Unsupported field"))
		}
	}
	return
}

func LoadConfiguration(path string) (config AppConfig, err error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("SERVER_IP", "127.0.0.1")
	v.SetDefault("SERVER_PORT", "8080")
	v.SetDefault("REFRESH_INTERVAL", 60)
	v.SetDefault("URLS", "http://opendata.digital.gov.ru/downloads/ABC-3xx.csv,http://opendata.digital.gov.ru/downloads/ABC-4xx.csv,http://opendata.digital.gov.ru/downloads/ABC-8xx.csv,http://opendata.digital.gov.ru/downloads/DEF-9xx.csv")
	v.SetConfigFile(path)
	_ = v.ReadInConfig()
	err = v.Unmarshal(&config)
	return config, err
}

func main() {
	var configPath string
	pflag.StringVarP(&configPath, "config", "c", ".", "Path to the configuration file")
	pflag.Parse()

	var logger, _ = zap.NewProduction()
	var log = logger.Sugar()

	log.Info("Reading configuration: " + configPath)
	appCfg, err := LoadConfiguration(configPath)
	if err != nil {
		log.Errorf("Can't read configuration. ERROR: %s", err)
		os.Exit(1)
	}
	log.Info("appCfg.BindIP ", appCfg.BindIP, " ", appCfg.URLs)
	log.Info("Run syncing goroutine")
	appCfg.registry = &rsvz_checker.Registry{AllCodes: map[int][]rsvz_checker.Code{}}
	appCfg.log = log
	go appCfg.registry.RegistryProcessing(appCfg.URLs, appCfg.RefreshInterval, log)

	log.Info("Complete")
	log.Infof("Service is running on %s:%s", appCfg.BindIP, appCfg.BindPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/check/", appCfg.ServeHTTP)
	err = http.ListenAndServe(appCfg.BindIP+":"+appCfg.BindPort, mux)
	if err != nil {
		log.Fatalf("Can't start http listene. Error: %s", err)
		os.Exit(1)
	}

}
