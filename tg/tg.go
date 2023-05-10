package tg

import (
	"fmt"
	"io/ioutil"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stnokott/telescan/scanner"
	"github.com/stnokott/telescan/scansession"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

const (
	scanFirstPage = "🖨 Scan first page"
	scanNextPage  = "🖨 Scan another page"
	finishScan    = "✅ Save"
	yesSend       = "✅ Yes, send it!"
	noSend        = "❌ No, just save it"
	cancel        = "❌ Cancel"
)

type TGConfig struct {
	APIKey         string
	Users          []string
	SendScanToChat bool
}

type TG struct {
	config TGConfig
	scm    *scanner.Manager
	ssm    *scansession.Manager
	bot    *tgbotapi.BotAPI
	logger *zap.SugaredLogger
}

func Init(
	config TGConfig,
	scm *scanner.Manager,
	ssm *scansession.Manager,
	logger *zap.SugaredLogger) *TG {
	if config.APIKey == "" {
		logger.Warnf("Telegram API key not set, not starting Telegram bot")
		return nil
	}

	t := TG{
		config: config,
		logger: logger,
		scm:    scm,
		ssm:    ssm,
	}

	bot, err := tgbotapi.NewBotAPI(t.config.APIKey)
	if err != nil {
		t.logger.Panicf("Unable to start Telegram bot: %s", err)
		return nil
	}

	t.bot = bot
	return &t
}

func (t *TG) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)

	for update := range updates {
		t.logger.Debugw(
			"Received message",
			"user", update.Message.From.UserName,
			"message", update.Message.Text,
		)

		if update.Message == nil {
			continue
		}

		if !slices.Contains(t.config.Users, update.Message.From.UserName) {
			t.logger.Debugw(
				"Unauthorized user detected",
				"username", update.Message.From.UserName,
				"id", update.Message.From.ID,
			)
			continue
		}

		ss := t.ssm.ScanSession(
			update.Message.From.UserName,
			update.Message.Chat.ID,
		)

		t.handleScanSession(ss, update)
	}
}

func (t *TG) handleScanSession(ss *scansession.ScanSession, u tgbotapi.Update) {
	switch u.Message.Text {
	case scanFirstPage, scanNextPage:
		t.handleScanRequest(ss, u)
	case finishScan:
		t.handleFinishScanRequest(ss, u)
	case yesSend, noSend:
		t.handleSendScanRequest(ss, u)
	case cancel:
		t.ssm.RemoveScanSession(u.Message.From.UserName)
		t.sendMsg(u.Message.Chat.ID, "✅ Scan cancelled.")
	default:
		if ss.NumImages() == 0 {
			t.sendMsgWithKB(u.Message.Chat.ID,
				"Welcome. Insert the first page and press Scan.",
				ss)
		} else {
			t.sendMsgWithKB(u.Message.Chat.ID,
				"Insert the next page and press Scan.",
				ss)
		}
	}
}

func (t *TG) handleFinishScanRequest(ss *scansession.ScanSession, u tgbotapi.Update) {
	t.sendMsg(u.Message.Chat.ID, "⌛ Finishing scan...")

	fileName, err := ss.WriteFinal()
	if err != nil {
		t.logAndReportErrorToUser(u, "Telescan was unable to write the pdf", err)
		return
	}
	if !t.config.SendScanToChat {
		t.sendMsg(u.Message.Chat.ID,
			fmt.Sprintf("✅ Scan finished. Wrote file `%s`.", fileName),
		)
		t.ssm.RemoveScanSession(u.Message.From.UserName)
	} else {
		t.sendMsgWithSendFileConfirmation(u.Message.Chat.ID,
			fmt.Sprintf("✅ Scan finished. Wrote file `%s`. Would you like a copy?", fileName))
	}
}

func (t *TG) handleSendScanRequest(ss *scansession.ScanSession, u tgbotapi.Update) {
	if u.Message.Text == yesSend {
		t.sendScanToChat(u, ss.Filename(), ss.FullPathToScan())
	}

	t.ssm.RemoveScanSession(u.Message.From.UserName)

	t.sendMsg(u.Message.Chat.ID, "✅ Session closed. See you later!")

}

func (t *TG) sendScanToChat(u tgbotapi.Update, fn, fp string) {
	fread, err := ioutil.ReadFile(fp)
	if err != nil {
		t.logAndReportErrorToUser(u, "Telescan was Unable to read the pdf.", err)
	}

	doc := tgbotapi.NewDocument(u.Message.Chat.ID, tgbotapi.FileBytes{
		Name:  fn,
		Bytes: fread,
	})
	t.bot.Send(doc)
}

func (t *TG) handleScanRequest(ss *scansession.ScanSession, u tgbotapi.Update) {
	t.sendMsg(u.Message.Chat.ID, "⌛ Scanning page, please wait...")

	scanner, err := t.scm.GetScanner()
	if err != nil {
		t.logAndReportErrorToUser(u, "Telescan couldn't grab a scanner to scan with", err)
		return
	}

	bytes, err := scanner.Scan()
	if err != nil {
		t.logAndReportErrorToUser(u, "Telescan couldn't use this scanner", err)
		return
	}

	ss.AddImage(bytes)
	t.sendMsgWithKB(
		u.Message.Chat.ID,
		fmt.Sprintf("✅ Scanned page %d.", ss.NumImages()),
		ss,
	)
}

func (t *TG) sendMsgWithKB(chatID int64, text string, ss *scansession.ScanSession) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	if ss.NumImages() == 0 {
		msg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton(scanFirstPage),
			),
		)
	} else {
		msg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton(scanNextPage),
				tgbotapi.NewKeyboardButton(finishScan),
				tgbotapi.NewKeyboardButton(cancel),
			),
		)
	}
	_, err := t.bot.Send(msg)
	if err != nil {
		t.logger.Errorw("Unable to send message", "error", err)
	}
	return err
}

func (t *TG) sendMsgWithSendFileConfirmation(chat int64, text string) error {
	msg := tgbotapi.NewMessage(chat, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(yesSend),
			tgbotapi.NewKeyboardButton(noSend),
		),
	)
	_, err := t.bot.Send(msg)
	if err != nil {
		t.logger.Errorw("Unable to send message", "error", err)
	}
	return err
}

func (t *TG) sendMsg(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	_, err := t.bot.Send(msg)
	if err != nil {
		t.logger.Errorw("Unable to send message", "error", err)
	}
	return err
}

func (t *TG) logAndReportErrorToUser(u tgbotapi.Update, friendlyError string, err error) {
	t.logger.Warnw("There was an error while dealing with a message",
		"user", u.Message.From.UserName,
		"message", u.Message.Text,
		"error", err,
		"friendly_error", friendlyError)

	t.sendMsg(u.Message.Chat.ID,
		fmt.Sprintf("❌ %s. The error was `%s`", friendlyError, err.Error()))

}
