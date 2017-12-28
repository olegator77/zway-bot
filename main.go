package main

import (
	"log"
	"net/http"
)

const (
	zwayURL      = "http://192.168.1.65:8083/ZAutomation/api/v1"
	zwayPassword = "admin"
	zwayLogin    = "admin"
)

var zway *ZWay
var cmd *CmdProcessor

func main() {
	zway = NewZWay(zwayURL)
	cmd = NewCmdProcessor()

	if err := zway.Auth(zwayLogin, zwayPassword); err != nil {
		log.Fatalf("Can't auth to zway: %s", err.Error())
	}

	devices, err := zway.Devices(false)
	if err != nil {
		log.Fatalf("Can't get devices from zway: %s", err.Error())
	}

	locations, err := zway.Locations(false)
	if err != nil {
		log.Fatalf("Can't get locations from zway: %s", err.Error())
	}

	for _, loc := range locations {
		log.Printf("%d\t%s\n", loc.ID, loc.Title)
		cmd.AddLocation(loc.ID, loc.Title)
	}

	for _, d := range devices {
		log.Printf("%s\t%s\t%s\t%d\t%d\n", d.ID, d.DeviceType, d.Metrics.Title, int(d.Metrics.Level), d.Location)
		cmd.AddDevice(d.ID, d.Metrics.Title, d.DeviceType, d.Location)
	}

	http.HandleFunc("/speech_action", func(w http.ResponseWriter, r *http.Request) {
		phrase := r.FormValue("text")
		log.Printf("%s -> %s\n", r.URL, phrase)

		devID, cmd := cmd.ProcessPhrase(phrase)

		if cmd != nil {
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
			log.Printf("Applying default command to device: '%s'", devID)
			zway.ControlToggle(devID)
		} else {
			log.Printf("Can't execute action")
		}

	})

	http.ListenAndServe(":8000", nil)
}
