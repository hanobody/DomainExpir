package telegram

import (
	"DomainC/config"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Button struct {
	Text         string
	CallbackData string
}

var Bot *tgbotapi.BotAPI

func Init() error {
	token := config.Cfg.Telegram.BotToken
	if token == "" {
		return nil
	}
	b, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return err
	}
	Bot = b
	Bot.Debug = false
	log.Printf("已登录 Telegram: %s", Bot.Self.UserName)
	return nil
}

func SendAlert(chatID int64, msg string) {
	message := tgbotapi.NewMessage(chatID, msg)
	message.ParseMode = "Markdown"
	Bot.Send(message)
}

func SendTelegramAlert(msg string) {
	chatID := int64(config.Cfg.Telegram.ChatID)
	message := tgbotapi.NewMessage(chatID, msg)
	message.ParseMode = "Markdown"
	Bot.Send(message)
}

func SendTelegramAlertWithButtons(msg string, buttons [][]Button) {
	chatID := int64(config.Cfg.Telegram.ChatID)
	message := tgbotapi.NewMessage(chatID, msg)

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, r := range buttons {
		var row []tgbotapi.InlineKeyboardButton
		for _, b := range r {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(b.Text, b.CallbackData))
		}
		rows = append(rows, row)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	message.ReplyMarkup = keyboard
	Bot.Send(message)
}

func StartListener(handleCallback func(data string, user *tgbotapi.User)) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := Bot.GetUpdatesChan(u)
	for up := range updates {
		if up.CallbackQuery != nil {
			handleCallback(up.CallbackQuery.Data, up.CallbackQuery.From)
			cb := tgbotapi.NewCallback(up.CallbackQuery.ID, "操作已收到")
			Bot.Send(cb)
		}
	}
}
