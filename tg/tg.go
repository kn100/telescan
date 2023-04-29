package tg

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kn100/telescan/scansession"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

const (
	createScanSession = "🖨 Scan Document"
	scanPage          = "📷 Scan"
	finishScan        = "✅ Finish"
	cancel            = "❌ Cancel"
)

type TG struct {
	bot               *tgbotapi.BotAPI
	apiKey            string
	logger            *zap.SugaredLogger
	authorizedUsers   []string
	activeScanSession *scansession.ScanSession
	scannerOverride   string
	tmpDir            string
	finalDir          string
}

func Init(apiKey string,
	authorizedUsers []string,
	tmpDir, finalDir, scannerOverride string,
	logger *zap.SugaredLogger) *TG {
	t := TG{}
	t.apiKey = apiKey
	t.tmpDir = tmpDir
	t.finalDir = finalDir
	t.scannerOverride = scannerOverride
	t.logger = logger
	t.authorizedUsers = authorizedUsers

	t.logger.Debug("Starting Telegram bot")
	if t.apiKey == "" {
		t.logger.Warnf("Telegram API key not set, not starting Telegram bot")
		return nil
	}

	bot, err := tgbotapi.NewBotAPI(t.apiKey)
	if err != nil {
		t.logger.Warnf("Unable to start Telegram bot: %s", err)
		return nil
	}
	t.bot = bot
	return &t
}

func (t *TG) Start() {
	t.logger.Debug("Starting Telegram bot")
	if t.apiKey == "" {
		t.logger.Warnf("Telegram API key not set, not starting Telegram bot")
		return
	}

	bot, err := tgbotapi.NewBotAPI(t.apiKey)
	if err != nil {
		t.logger.Warnf("Unable to start Telegram bot: %s", err)
		return
	}
	t.bot = bot

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)

	for update := range updates {
		t.logger.Debugw("Received message",
			"user", update.Message.From.UserName,
			"message", update.Message.Text)

		if update.Message == nil ||
			!slices.Contains(t.authorizedUsers, update.Message.From.UserName) {
			t.logger.Debugw("Ignoring message from unauthorized user",
				"user", update.Message.From.UserName)
			continue
		}
		t.logger.Debugw("active scan session", "session", t.activeScanSession)
		if t.activeScanSession == nil {
			// There is no active scan session, and the user probably wants one
			t.newScanSession(update)
			continue
		} else if t.activeScanSession.UserName == update.Message.From.UserName {
			// There is an active scan session we own
			t.logger.Debugw("Handling scan session message",
				"user", update.Message.From.UserName)
			t.handleScanSession(update)
			continue
		} else if t.activeScanSession.UserName != "" &&
			// There is an active scan session we don't own
			t.activeScanSession.UserName != update.Message.From.UserName {
			t.logAndReportErrorToUser(update, "Scanner is in use at the moment", err)
			continue
		}
	}
}

// newScanSession creates a new scan session for the user
func (t *TG) newScanSession(update tgbotapi.Update) {
	if update.Message.Text != createScanSession {
		t.logger.Debug("Unknown command")
		t.handleUnknownCommand(update)
		return
	}
	scanSession, err := scansession.Init(
		update.Message.From.UserName,
		t.scannerOverride,
		t.tmpDir,
		t.finalDir,
		t.logger,
	)
	if err != nil {
		t.logAndReportErrorToUser(update, "Unable to create scan session", err)
		return
	}

	t.activeScanSession = scanSession
	t.logger.Debugw("Created new scan session",
		"user", update.Message.From.UserName)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You have the scanner.")
	msg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(scanPage),
			tgbotapi.NewKeyboardButton(cancel),
		),
	)
	t.bot.Send(msg)
}

func (t *TG) handleScanSession(update tgbotapi.Update) {
	switch update.Message.Text {
	case scanPage:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			"Scanning page...")
		_, err := t.bot.Send(msg)
		if err != nil {
			t.logAndReportErrorToUser(update, "Unable to send message", err)
			return
		}
		t.logger.Debugf("Scanning page %d", t.activeScanSession.NumberOfPagesScanned())
		err = t.activeScanSession.Scan()
		if err != nil {
			t.logger.Debugf("Unable to scan page: %s", err.Error())
			t.logAndReportErrorToUser(update, "Unable to scan page", err)
			return
		}
		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("Scanned page %d. Please scan another page or finish.",
				t.activeScanSession.NumberOfPagesScanned()))
		msg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton(scanPage),
				tgbotapi.NewKeyboardButton(finishScan),
				tgbotapi.NewKeyboardButton(cancel),
			),
		)
		t.bot.Send(msg)
		return
	case finishScan:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			"Finishing scan...")
		t.bot.Send(msg)
		fileName, err := t.activeScanSession.WriteFinal()
		if err != nil {
			t.logAndReportErrorToUser(update, "Unable to write final scan", err)
			return
		}
		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("Scan finished. Wrote file `%s`.", fileName))
		t.bot.Send(msg)
		t.activeScanSession = nil
		t.handleUnknownCommand(update)
	case cancel:
		t.activeScanSession.Cancel()
		t.activeScanSession = nil
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Scan cancelled.")
		t.bot.Send(msg)
		t.handleUnknownCommand(update)
	default:
		t.handleUnknownCommand(update)
	}
}

func (t *TG) handleUnknownCommand(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID,
		"Welcome. Please use the buttons below to start scanning.")
	msg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(createScanSession),
		),
	)
	t.bot.Send(msg)
}

func (t *TG) logAndReportErrorToUser(
	update tgbotapi.Update, friendlyError string, err error) {
	t.logger.Warnw("There was an error while dealing with a message",
		"user", update.Message.From.UserName,
		"message", update.Message.Text,
		"error", err,
		"friendlyError", friendlyError)

	msg := tgbotapi.NewMessage(update.Message.Chat.ID,
		fmt.Sprintf("Sorry, there was an error: %s - %s", friendlyError, err.Error()))
	t.bot.Send(msg)
}
