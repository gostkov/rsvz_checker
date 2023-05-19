package main

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"os"
	rsvz_checker "rsvz_checker/pkg"
	"strconv"
)

const ErrorForParseText = "Не удалось распознать номер"

type BotService struct {
	BotToken        string   `mapstructure:"BOT_TOKEN"`
	DebugMode       bool     `mapstructure:"BOT_DEBUG"`
	RefreshInterval int      `mapstructure:"REFRESH_INTERVAL"`
	URLs            []string `mapstructure:"URLS"`
	Bot             *tgbotapi.BotAPI
	log             *zap.SugaredLogger
	registry        *rsvz_checker.Registry
}

func (bs *BotService) runBotProcessing() error {

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.AllowedUpdates = []string{"message"}
	updates := bs.Bot.GetUpdatesChan(u)

	userName := ""
	receivedText := ""
	textForSend := ""
	for update := range updates {
		if update.Message == nil {
			continue
		}
		receivedText = update.Message.Text
		userName = update.Message.Chat.UserName
		bs.log.Infof("Received msg. Name: [%s] Text: %s", userName, receivedText)

		phone, err := rsvz_checker.ParseRawTgText(receivedText)
		if err != nil {
			textForSend = ErrorForParseText
		} else {
			parsedResult, err := rsvz_checker.IncomingRFPhoneProcessing(strconv.Itoa(phone))
			if err != nil {
				textForSend = ErrorForParseText
			} else {
				parsedResult.Operator, parsedResult.Region = bs.registry.SearchCodeByPrefixAndPhone(parsedResult.Code, parsedResult.Phone)
				bs.log.Infof("Parced: code: %d, phone: %d, operator: %s, region: %s",
					parsedResult.Code, parsedResult.Phone, parsedResult.Operator, parsedResult.Region)
				textForSend = "По номеру: " + strconv.Itoa(phone) + " найдены следующие данные:\n" + "Код: " +
					strconv.Itoa(parsedResult.Code) + "\nНомер: " + strconv.Itoa(parsedResult.Phone) + "\nОператор: " + parsedResult.Operator +
					"\nРегион: " + parsedResult.Region
			}
		}
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, textForSend)
		msg.ReplyToMessageID = update.Message.MessageID
		_, err = bs.Bot.Send(msg)
		if err != nil {
			bs.log.Errorf("Failed send message to user[%s]: %s", userName, err.Error())
		}

	}
	return nil
}

func prepareBotService(configPath string) (*BotService, error) {
	botService, err := LoadConfiguration(configPath)
	if err != nil {
		return nil, err
	}
	if botService.BotToken == "" {
		return nil, errors.New("bot token can't be empty")
	}
	bot, err := tgbotapi.NewBotAPI(botService.BotToken)
	if err != nil {
		return nil, err
	}
	bot.Debug = botService.DebugMode
	botService.Bot = bot
	return &botService, nil
}

func LoadConfiguration(path string) (config BotService, err error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("BOT_TOKEN", "6118716272:AAFNDSJu0xTfqmpMO-P_Q6VNWDIjkC-5bAs")
	v.SetDefault("BOT_DEBUG", false)
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

	botService, err := prepareBotService(configPath)
	if err != nil {
		log.Fatalf("Can't run bot. Error: %s", err.Error())
		os.Exit(2)
	}
	botService.log = log
	botService.registry = &rsvz_checker.Registry{AllCodes: map[int][]rsvz_checker.Code{}}
	go botService.registry.RegistryProcessing(botService.URLs, botService.RefreshInterval, log)
	log.Info("starting bot")
	err = botService.runBotProcessing()
	if err != nil {
		log.Fatalf("Can't run update listiner: %s", err.Error())
		os.Exit(3)
	}
}
