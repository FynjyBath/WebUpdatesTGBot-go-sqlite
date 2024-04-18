package tgbot

import (
	"log"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type TGBot struct {
	Bot *tgbotapi.BotAPI
}

func (tgbot *TGBot) Init(tgbotkey string) {
	bot, err := tgbotapi.NewBotAPI(tgbotkey)
	if err != nil {
		log.Fatal(err)
	}
	bot.Debug = true
	tgbot.Bot = bot
}

func RemoveNonUTF8Runes(s string) string {
	var valid []rune
	for i, w := 0, 0; i < len(s); i += w {
		runeValue, width := utf8.DecodeRuneInString(s[i:])
		if runeValue != utf8.RuneError {
			valid = append(valid, runeValue)
		}
		w = width
	}
	return string(valid)
}

func (bot *TGBot) SendMessage(id int, message string) tgbotapi.Message {
	message = RemoveNonUTF8Runes(message)
	msg := tgbotapi.NewMessage(int64(id), message)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	msg_sended, err := bot.Bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
	return msg_sended
}
