package tg

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	settings = "⚙️ Settings"
	back     = "⬅️ Back"
)

func (t *TG) sendSettingsKeyboard(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Scanner Settings")
	msg.ParseMode = tgbotapi.ModeMarkdown

	var rows [][]tgbotapi.KeyboardButton
	rows = append(rows, tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(back),
	))

	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(rows...)

	t.bot.Send(msg)
}
