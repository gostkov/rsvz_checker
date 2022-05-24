package main

import (
	"bufio"
	"errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var logger, _ = zap.NewProduction()
var zaplog = logger.Sugar()

type phonesHandler struct {
	AllCodes map[int][]Code
}

type Code struct {
	startRange int
	endRange   int
	pop        string
	region     string
}

type AppConfig struct {
	RefreshInterval int      `mapstructure:"REFRESH_INTERVAL"`
	BindPort        string   `mapstructure:"SERVER_PORT"`
	BindIP          string   `mapstructure:"SERVER_IP"`
	URLs            []string `mapstructure:"URLS"`
}

func (h *phonesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO check
	w.Write([]byte("OK "))
	str := strconv.Itoa(len(h.AllCodes))
	zaplog.Infow("TEST Serve:" + str)
}

func LoadConfiguration(path string) (config AppConfig, err error) {
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

func downloadFiles(urls []string) ([]string, error) {
	downloadedFiles := make([]string, len(urls))
	errGrp := new(errgroup.Group)

	for i, url := range urls {
		func(idx int, url string) {
			errGrp.Go(func() error {
				out, err := os.Create(strconv.Itoa(idx) + "_data.csv")
				if err != nil {
					return err
				}
				defer out.Close()
				resp, err := http.Get(url)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					return errors.New("file not found " + url)
				}
				if _, err = io.Copy(out, resp.Body); err != nil {
					return err
				}
				downloadedFiles[idx] = strconv.Itoa(idx) + "_data.csv"
				return err
			})
		}(i, url)
	}
	err := errGrp.Wait()
	if err != nil {
		return []string{}, err
	}
	return downloadedFiles, err
}

func fileParsing(f string, prefixMap map[int][]Code) error {
	file, err := os.Open(f)
	if err != nil {
		return err
	}
	defer file.Close()

	row := make([]string, 6)
	scanner := bufio.NewScanner(file)
	scanner.Scan() // skip first line
	code, start, end := 0, 0, 0

	for scanner.Scan() {
		row = strings.Split(scanner.Text(), ";")
		code, err = strconv.Atoi(row[0])
		if err != nil {
			return err
		}
		start, err = strconv.Atoi(row[1])
		end, err = strconv.Atoi(row[2])

		prefixMap[code] = append(prefixMap[code], Code{
			startRange: start,
			endRange:   end,
			pop:        row[4],
			region:     row[5],
		})
	}
	return nil
}

func processingPhones(config AppConfig, handler *phonesHandler) {
	for {
		zaplog.Infow("starting file download")
		files, err := downloadFiles(config.URLs)
		if err != nil {
			zaplog.Errorw("some files didn't download. Stop processing. Waiting next iteration")
			time.Sleep(time.Minute * time.Duration(config.RefreshInterval))
			continue
		}
		// files := []string{"0_data.csv", "1_data.csv", "2_data.csv", "3_data.csv"}
		zaplog.Infof("saved files: %+v", files)
		for _, f := range files {
			zaplog.Infow("start parsing file: " + f)
			err = fileParsing(f, handler.AllCodes)
			if err != nil {
				zaplog.Errorf("Error occurred during file parsing. File: %s. Error: %s", f, err)
			}
		}
		zaplog.Infof("Parsing completed. Next sync after %d minutes", config.RefreshInterval)
		time.Sleep(time.Minute * time.Duration(config.RefreshInterval))
	}
}

func main() {
	var configPath string
	pflag.StringVarP(&configPath, "config", "c", ".", "Path to configuration file")
	pflag.Parse()

	zaplog.Infow("Reading configuration: " + configPath)
	cfg, err := LoadConfiguration(configPath)
	if err != nil {
		log.Println("ERROR: ", err)
		os.Exit(1)
	}

	allPrefixes := &phonesHandler{AllCodes: map[int][]Code{}}
	zaplog.Infow("Run syncing goroutine")
	go processingPhones(cfg, allPrefixes)

	zaplog.Infof("Service is running on %s:%s", cfg.BindIP, cfg.BindPort)

	http.Handle("/check", allPrefixes)
	err = http.ListenAndServe(cfg.BindIP+":"+cfg.BindPort, nil)
	if err != nil {
		zaplog.Fatalf("Fatal error: %s", err)
		os.Exit(1)
	}
}
