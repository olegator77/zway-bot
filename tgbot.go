package main

import (
	"fmt"
	"log"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

func StartTgBot() {
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

				ans := ""
				switch update.Message.Text {
				case "/start":
					ans = "Привет, я умею управлять умным домом."
				case "/rooms":
					ans = ""
					locs, _ := zway.Locations(false)
					for _, loc := range locs {
						ans += loc.Title + "\n"
					}
				case "/devices":
					ans = ""
					devs, _ := zway.Devices(false)
					for _, dev := range devs {
						ans += fmt.Sprintf("%s - %d\n", dev.Metrics.Title, int(dev.Metrics.Level))
					}

				default:
					ans = runCommand(update.Message.Text, userName)
				}
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, ans)
				//			msg.ReplyToMessageID = update.Message.MessageID
				bot.Send(msg)
			}
		}()
	}
}
