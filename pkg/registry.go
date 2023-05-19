package rsvz_checker

import (
	"bufio"
	"errors"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DirForTmpCsv = "/tmp/"

type Registry struct {
	AllCodes map[int][]Code
	mu       sync.Mutex
}

type Code struct {
	StartRange int
	EndRange   int
	Pop        string
	Region     string
}

func (r *Registry) SearchCodeByPrefixAndPhone(code, phone int) (string, string) {
	var (
		operator string
		region   string
	)

	for c := range r.AllCodes[code] {
		if phone >= r.AllCodes[code][c].StartRange && phone <= r.AllCodes[code][c].EndRange {
			operator = strings.Replace(r.AllCodes[code][c].Pop, `"`, "", 3)
			region = r.AllCodes[code][c].Region
			break
		}
	}
	return operator, region
}

func (r *Registry) downloadFiles(urls []string) ([]string, error) {
	downloadedFiles := make([]string, len(urls))
	errGrp := new(errgroup.Group)

	for i, url := range urls {
		func(idx int, url string) {
			errGrp.Go(func() error {
				resp, err := http.Get(url)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					return errors.New("file not found " + url)
				}
				out, err := os.Create(DirForTmpCsv + strconv.Itoa(idx) + "_data.csv")
				if err != nil {
					return err
				}
				defer out.Close()
				if _, err = io.Copy(out, resp.Body); err != nil {
					return err
				}
				downloadedFiles[idx] = DirForTmpCsv + strconv.Itoa(idx) + "_data.csv"
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

func (r *Registry) registryFileParser(f string) (error, map[int][]Code) {
	prefixMap := make(map[int][]Code, 10000)
	file, err := os.Open(f)
	if err != nil {
		return err, nil
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
			return err, nil
		}
		start, err = strconv.Atoi(row[1])
		end, err = strconv.Atoi(row[2])

		prefixMap[code] = append(prefixMap[code], Code{
			StartRange: start,
			EndRange:   end,
			Pop:        row[4],
			Region:     row[5],
		})
	}
	return nil, prefixMap
}

// RegistryProcessing функция загружает из минцифры файлы с нумерацией, парсит их и сохраняет в мапу AllCodes
func (r *Registry) RegistryProcessing(urls []string, refreshInterval int, log *zap.SugaredLogger) {
	isDryRun := true
	var err error
	if _, err = os.Stat(DirForTmpCsv); err != nil {
		os.Mkdir(DirForTmpCsv, 0750)
	}
	files := make([]string, 0, 5)

	for {
		files = nil
		log.Info("Starting files download")
		// если сервис был перезапущен, то скорее всего файлы уже есть
		if isDryRun {
			var testCsvFile string
			for i := 0; i < len(urls); i++ {
				testCsvFile = DirForTmpCsv + strconv.Itoa(i) + "_data.csv"
				if _, err = os.Stat(testCsvFile); err == nil {
					files = append(files, testCsvFile)
				}
			}
			isDryRun = false
		}
		if len(files) == 0 {
			files, err = r.downloadFiles(urls)
			if err != nil {
				log.Error("some files didn't download. Stop processing. Waiting next iteration")
				time.Sleep(time.Minute * time.Duration(refreshInterval))
				continue
			}
		}
		log.Infof("Saved files: %+v", files)
		tmpRegistry := make(map[int][]Code)
		parsedCodes := make(map[int][]Code)
		for _, f := range files {
			log.Info("Start parsing file: " + f)
			err, parsedCodes = r.registryFileParser(f)
			if err != nil {
				log.Errorf("Error occurred during file parsing. File: %s. Error: %s", f, err)
			}
			maps.Copy(tmpRegistry, parsedCodes)
		}
		r.mu.Lock()
		maps.Copy(r.AllCodes, tmpRegistry)
		r.mu.Unlock()
		log.Infof("Parsing completed. Next sync after %d minutes", refreshInterval)
		time.Sleep(time.Minute * time.Duration(refreshInterval))
	}
}
