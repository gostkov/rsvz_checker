package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"io"
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

type ParsedResult struct {
	Code     int    `json:"code"`
	FullNum  string `json:"full_num"`
	Operator string `json:"operator"`
	Region   string `json:"region"`
}

type AppConfig struct {
	RefreshInterval int      `mapstructure:"REFRESH_INTERVAL"`
	BindPort        string   `mapstructure:"SERVER_PORT"`
	BindIP          string   `mapstructure:"SERVER_IP"`
	URLs            []string `mapstructure:"URLS"`
}

/*
	TODO urlencode +
Docker
*/

func (h *phonesHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		zaplog.Infow("incoming request: " + req.URL.RawQuery)

		field := req.URL.Query().Get("field")
		num := strings.Replace(req.URL.Query().Get("num"), "+", ``, -1)
		num = strings.Replace(req.URL.Query().Get("num"), " ", ``, -1)
		code, phone, err := incomingPhoneProcessing(num)
		if err != nil {
			zaplog.Errorf("phone processing error: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "Bad request!")
			return
		}
		zaplog.Infof("Parced: code %d phone %d", code, phone)
		res := &ParsedResult{
			Code:    code,
			FullNum: num,
		}
		for c := range h.AllCodes[code] {
			if phone >= h.AllCodes[code][c].startRange && phone <= h.AllCodes[code][c].endRange {
				zaplog.Infof("Found: %+v", h.AllCodes[code][c])
				res.Operator = strings.Replace(h.AllCodes[code][c].pop, `"`, "", 3)
				res.Region = h.AllCodes[code][c].region
				break
			}
		}
		if field == "" {
			jsonResp, _ := json.Marshal(res)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, err = w.Write(jsonResp)
		} else {
			switch field {
			case "code":
				w.Write([]byte(strconv.Itoa(res.Code)))
			case "full_name":
				w.Write([]byte(res.FullNum))
			case "operator":
				w.Write([]byte(res.Operator))
			case "region":
				w.Write([]byte(res.Region))
			default:
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Unsupported field"))
			}
		}
		return
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = fmt.Fprintf(w, "Not supported method!")
	}
}

func incomingPhoneProcessing(num string) (code int, phone int, err error) {
	if len(num) == 10 {
		num = "7" + num
	}
	if len(num) < 11 {
		return 0, 0, errors.New("num too short")
	}
	tmp := num[1:4]
	code, err = strconv.Atoi(tmp)
	if err != nil {
		return 0, 0, err
	}
	tmp = num[4:]
	phone, err = strconv.Atoi(tmp)
	return code, phone, err
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
		zaplog.Errorf("ERROR: %s", err)
		os.Exit(1)
	}

	allPrefixes := &phonesHandler{AllCodes: map[int][]Code{}}
	zaplog.Infow("Run syncing goroutine")
	go processingPhones(cfg, allPrefixes)

	zaplog.Infof("Service is running on %s:%s", cfg.BindIP, cfg.BindPort)

	http.Handle("/check/", allPrefixes)
	err = http.ListenAndServe(cfg.BindIP+":"+cfg.BindPort, nil)
	if err != nil {
		zaplog.Fatalf("Fatal error: %s", err)
		os.Exit(1)
	}
}
