package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"gopkg.in/telegram-bot-api.v4"
)

const (
	zwayURL      = "http://192.168.1.65:8083/ZAutomation/api/v1"
	zwayPassword = "admin"
	zwayLogin    = "admin"
)

var zway *ZWay
var cmd *CmdProcessor

func main() {

	initAll()

	bot, err := tgbotapi.NewBotAPI("-----")
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

			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
			ans := runCommand(update.Message.Text)

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, ans)
			//			msg.ReplyToMessageID = update.Message.MessageID

			bot.Send(msg)
		}
	}()

	http.HandleFunc("/speech_action", func(w http.ResponseWriter, r *http.Request) {
		phrase := r.FormValue("text")
		log.Printf("%s -> %s\n", r.URL, phrase)

		runCommand(phrase)
	})

	http.ListenAndServe(":8000", nil)
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

func runCommand(phrase string) (msg string) {

	devID, locID, cmd := cmd.ProcessPhrase(phrase)
	if cmd != nil {
		msg = fmt.Sprintf("Выполняю '%s' на '%s' в '%s'", cmd.Words, zway.DeviceTitle(devID), zway.LocationTitle(locID))
		log.Printf("Applying command '%s' to device: '%s'", cmd.Words, devID)
		//			log.Printf("%s\t%s\t%s\t%d\t%s\n", dev.ID, dev.DeviceType, dev.Metrics.Title, bestScore, bestCmd.Words)

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

		}
	} else if devID != "" {
		msg = fmt.Sprintf("Переключаю устройство '%s'", zway.DeviceTitle(devID))
		log.Printf("Applying default command to device: '%s'", devID)
		zway.ControlToggle(devID)
	} else {
		msg = fmt.Sprintf("Не понял, что надо")
		log.Printf("Can't execute action")
	}
	return msg
}
