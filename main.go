package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

var zway *ZWay
var cmd *CmdProcessor
var zwayURL, zwayPassword, zwayLogin, tgBotToken, listenAddr, tgBotUsers, bindLocations string

func main() {
	flag.StringVar(&zwayURL, "zway-url", "http://127.0.0.1:8083/ZAutomation/api/v1", "URL to ZWay server")
	flag.StringVar(&zwayLogin, "zway-user", "admin", "User name for ZWay server")
	flag.StringVar(&zwayPassword, "zway-password", "admin", "Password for ZWay server")
	flag.StringVar(&tgBotToken, "tg-bot-token", "", "Telegram bot token")
	flag.StringVar(&tgBotUsers, "tg-bot-users", "", "Comma separated telegram users, who authorized to communicate with bot")
	flag.StringVar(&bindLocations, "bind-locations", "", "Comma separated bindings of sender's default locations, e.g 'olegator77=cabinet,192.168.1.101=hall")
	flag.StringVar(&listenAddr, "http-addr", ":8000", "HTTP listen address")
	flag.Parse()

	initAll()

	for _, ctxLocBind := range strings.Split(bindLocations, ",") {
		if len(ctxLocBind) == 0 {
			continue
		}
		locBind := strings.Split(ctxLocBind, "=")
		if len(locBind) != 2 {
			log.Fatalf("Invalid location binding: '%s'", bindLocations)
		}
		if !cmd.SetContextDefaultLocation(locBind[0], locBind[1]) {
			log.Fatalf("Can't bind context '%s': Not found location '%s'", locBind[0], locBind[1])
		} else {
			log.Printf("Binding '%s' as default location for '%s'", locBind[1], locBind[0])
		}
	}

	StartTgBot()

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

	devIDs, locIDs, cmd := cmd.ProcessPhrase(phrase, ctxName)
	devNames := ""
	for i, devID := range devIDs {
		if i != 0 {
			devNames += ","
		}
		devNames += zway.DeviceTitle(devID)
	}

	locNames := ""
	for i, locID := range locIDs {
		if i != 0 {
			locNames += ","
		}
		locNames += zway.LocationTitle(locID)
	}

	if cmd != nil {
		msg = fmt.Sprintf("Выполняю %s на %s", cmd.Words, devNames)
		if len(locNames) != 0 {
			msg += fmt.Sprintf(" в %s", locNames)
		}

		log.Printf("Applying command '%s' to device: '%s' in ctx %s", cmd.Words, devNames, ctxName)

		for _, devID := range devIDs {
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
		}
	} else if len(devIDs) != 0 {
		msg = fmt.Sprintf("Переключаю %s", devNames)
		log.Printf("Applying default command to device: '%s' in ctx %s", devNames, ctxName)
		for _, devID := range devIDs {
			zway.ControlToggle(devID)
		}
	} else {
		msg = fmt.Sprintf("Не понял команду")
		log.Printf("Can't execute action")
	}
	return msg
}
