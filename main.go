package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"gopkg.in/telegram-bot-api.v4"
)

var zway *ZWay
var cmd *CmdProcessor
var zwayURL, zwayPassword, zwayLogin, tgBotToken, listenAddr, tgBotUsers string

func main() {
	flag.StringVar(&zwayURL, "zway-url", "http://127.0.0.1:8083/ZAutomation/api/v1", "URL to ZWay server")
	flag.StringVar(&zwayLogin, "zway-user", "admin", "User name for ZWay server")
	flag.StringVar(&zwayPassword, "zway-password", "admin", "Password for ZWay server")
	flag.StringVar(&tgBotToken, "tg-bot-token", "", "Telegram bot token")
	flag.StringVar(&tgBotUsers, "tg-bot-users", "", "Comma sepearated telegram users, which authorized to communicate with bot")
	flag.StringVar(&listenAddr, "http-addr", ":8000", "HTTP listen address")
	flag.Parse()

	initAll()
	cmd.SetContextDefaultLocation("192.168.1.143", "кабинет")
	cmd.SetContextDefaultLocation("192.168.1.101", "гостинная")

	if len(tgBotToken) > 0 && len(tgBotUsers) > 0 {
		enabledUsers := make(map[string]bool)
		for _, userName := range strings.Split(tgBotUsers, ",") {
			enabledUsers[userName] = true
		}

		bot, err := tgbotapi.NewBotAPI(tgBotToken)
		if err != nil {
			log.Panic(err)
		}

		log.Printf("Authorized on account %s", bot.Self.UserName)

		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60

		go func() {
			updates, _ := bot.GetUpdatesChan(u)

			for update := range updates {
				if update.Message == nil {
					continue
				}

				userName := update.Message.From.UserName
				log.Printf("[%s] %s", userName, update.Message.Text)
				if _, found := enabledUsers[userName]; !found {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Refused command from unauthorized account"))
					continue
				}

				ans := runCommand(update.Message.Text, userName)

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, ans)
				//			msg.ReplyToMessageID = update.Message.MessageID
				bot.Send(msg)
			}
		}()
	}
	http.HandleFunc("/speech_action", func(w http.ResponseWriter, r *http.Request) {
		phrase := r.FormValue("text")
		log.Printf("%s -> %s\n", r.URL, phrase)
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		runCommand(phrase, host)
	})

	http.ListenAndServe(listenAddr, nil)
}

func initAll() {
	zway = NewZWay(zwayURL)
	cmd = NewCmdProcessor()

	if err := zway.Auth(zwayLogin, zwayPassword); err != nil {
		log.Fatalf("Can't auth to zway: %s", err.Error())
	}

	locations, err := zway.Locations(false)
	if err != nil {
		log.Fatalf("Can't get locations from zway: %s", err.Error())
	}

	for _, loc := range locations {
		title := cmd.AddLocation(loc.ID, loc.Title)
		log.Printf("%-4d %-10s (name=%s)\n", loc.ID, title, loc.Title)
	}

	devices, err := zway.Devices(false)
	if err != nil {
		log.Fatalf("Can't get devices from zway: %s", err.Error())
	}

	for _, d := range devices {
		title := cmd.AddDevice(d.ID, d.Metrics.Title, d.DeviceType, d.Location)
		log.Printf("%-27s %-17s %-10s %-16s (name='%s' lvl=%d)", d.ID, d.DeviceType, cmd.GetLocationTitle(d.Location), title, d.Metrics.Title, int(d.Metrics.Level))
	}

	zway.StartPolling(time.Duration(30 * time.Second))
}

func runCommand(phrase string, ctxName string) (msg string) {

	devID, locID, cmd := cmd.ProcessPhrase(phrase, ctxName)
	if cmd != nil {
		msg = fmt.Sprintf("Выполняю '%s' на '%s' в '%s'", cmd.Words, zway.DeviceTitle(devID), zway.LocationTitle(locID))
		log.Printf("Applying command '%s' to device: '%s' in ctx %s", cmd.Words, devID, ctxName)

		switch cmd.Command {
		case CommandOn:
			zway.ControlOn(devID)
		case CommandOff:
			zway.ControlOff(devID)
		case CommandRGB:
			rgb := cmd.CmdData.(CommandDataRGB)
			zway.ControlRGB(devID, rgb.R, rgb.G, rgb.B)
		case CommandDimmerDown:
			zway.ControlDimmerDown(devID)
		case CommandDimmerUp:
			zway.ControlDimmerUp(devID)
		case CommandDimmerMax:
			zway.ControlDimmerMax(devID)
		}
	} else if devID != "" {
		msg = fmt.Sprintf("Переключаю устройство '%s'", zway.DeviceTitle(devID))
		log.Printf("Applying default command to device: '%s' in ctx %s", devID, ctxName)
		zway.ControlToggle(devID)
	} else {
		msg = fmt.Sprintf("Не понял команду")
		log.Printf("Can't execute action")
	}
	return msg
}
