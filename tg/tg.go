package tg

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kn100/telescan/scanner"
	"github.com/kn100/telescan/scansession"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

const (
	scanFirstPage = "🖨 Scan first page"
	scanNextPage  = "🖨 Scan another page"
	finishScan    = "✅ Finish"
	cancel        = "❌ Cancel"
)

type TG struct {
	scm             *scanner.Manager
	ssm             *scansession.Manager
	bot             *tgbotapi.BotAPI
	apiKey          string
	logger          *zap.SugaredLogger
	authorizedUsers []string
}

func Init(
	apiKey string,
	authorizedUsers []string,
	scm *scanner.Manager,
	ssm *scansession.Manager,
	logger *zap.SugaredLogger) *TG {
	if apiKey == "" {
		logger.Warnf("Telegram API key not set, not starting Telegram bot")
		return nil
	}

	t := TG{
		apiKey:          apiKey,
		logger:          logger,
		authorizedUsers: authorizedUsers,
		scm:             scm,
		ssm:             ssm,
	}

	bot, err := tgbotapi.NewBotAPI(t.apiKey)
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

		if !slices.Contains(t.authorizedUsers, update.Message.From.UserName) {
			t.logger.Debugw(
				"Unauthorized user detected",
				"username", update.Message.From.UserName,
				"id", update.Message.From.ID,
			)
			continue
		}

		ss := t.ssm.ScanSession(update.Message.From.UserName, update.Message.Chat.ID)

		t.handleScanSession(ss, update)
	}
}

func (t *TG) handleScanSession(scanSession *scansession.ScanSession, update tgbotapi.Update) {
	switch update.Message.Text {
	case scanFirstPage, scanNextPage:
		t.handleScanRequest(scanSession, update)
	case finishScan:
		t.handleFinishScanRequest(scanSession, update)
	case cancel:
		t.ssm.RemoveScanSession(update.Message.From.UserName)
		t.sendMsg(update.Message.Chat.ID, "✅ Scan cancelled.")
	default:
		if scanSession.NumImages() == 0 {
			t.sendMsgWithKB(update.Message.Chat.ID,
				"Welcome. Insert the first page and press Scan.",
				scanSession)
		} else {
			t.sendMsgWithKB(update.Message.Chat.ID,
				"Insert the next page and press Scan.",
				scanSession)
		}
	}
}

func (t *TG) handleFinishScanRequest(scanSession *scansession.ScanSession, update tgbotapi.Update) {
	t.sendMsg(update.Message.Chat.ID, "⌛ Finishing scan...")

	fileName, err := scanSession.WriteFinal()
	if err != nil {
		t.logAndReportErrorToUser(update, "Telescan was Unable to write the pdf. Note that the files you scanned should still be present in the temporary directory, if they're important", err)
		return
	}
	t.ssm.RemoveScanSession(update.Message.From.UserName)

	t.sendMsg(update.Message.Chat.ID,
		fmt.Sprintf("✅ Scan finished. Wrote file `%s`.", fileName))
}

func (t *TG) handleScanRequest(scanSession *scansession.ScanSession, update tgbotapi.Update) {
	t.sendMsg(update.Message.Chat.ID, "⌛ Scanning page, please wait...")

	scanner, err := t.scm.GetScanner()
	if err != nil {
		t.logAndReportErrorToUser(update, "Telescan couldn't grab a scanner to scan with", err)
		return
	}

	bytes, err := scanner.Scan()
	if err != nil {
		t.logAndReportErrorToUser(update, "Telescan couldn't use this scanner", err)
		return
	}

	scanSession.AddImage(bytes)
	t.sendMsgWithKB(
		update.Message.Chat.ID,
		fmt.Sprintf("✅ Scanned page %d.", scanSession.NumImages()),
		scanSession,
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

func (t *TG) logAndReportErrorToUser(
	update tgbotapi.Update, friendlyError string, err error) {
	t.logger.Warnw("There was an error while dealing with a message",
		"user", update.Message.From.UserName,
		"message", update.Message.Text,
		"error", err,
		"friendly_error", friendlyError)

	t.sendMsg(update.Message.Chat.ID,
		fmt.Sprintf("❌ %s. The error was `%s`", friendlyError, err.Error()))

}
