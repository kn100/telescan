package tg

import (
	"fmt"
	"time"

	"github.com/brutella/dnssd"
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
	ScanGCTimeout     = 5 * time.Minute
)

type TG struct {
	bot             *tgbotapi.BotAPI
	apiKey          string
	logger          *zap.SugaredLogger
	authorizedUsers []string
	session         *scansession.ScanSession
	scannerOverride string
	tmpDir          string
	finalDir        string
	scanners        *[]*dnssd.BrowseEntry
}

func Init(
	scanners *[]*dnssd.BrowseEntry,
	authorizedUsers []string,
	apiKey, tmpDir, finalDir, scannerOverride string,
	logger *zap.SugaredLogger) *TG {
	if apiKey == "" {
		logger.Warnf("Telegram API key not set, not starting Telegram bot")
		return nil
	}

	t := TG{
		apiKey:          apiKey,
		tmpDir:          tmpDir,
		finalDir:        finalDir,
		scannerOverride: scannerOverride,
		logger:          logger,
		authorizedUsers: authorizedUsers,
		scanners:        scanners,
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
	go func() {
		t.logger.Infof("Starting scan job GC")
		for {
			time.Sleep(1 * time.Minute)
			t.logger.Debugw("Running scan job GC")
			if t.session != nil && t.session.ScanLastUpdated.Add(ScanGCTimeout).Before(time.Now()) {
				t.logger.Infow("Scan session expired. Expiring.", "session", t.session)
				t.handleUnknownCommand(t.session.ChatID)
				t.session = nil
			}
		}
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)

	for update := range updates {
		t.logger.Debugw("Received message", "user", update.Message.From.UserName, "message", update.Message.Text)
		t.logger.Debugw("Active scan session", "session", t.session)

		// Handle empty messages as well as messages from unauthorized users
		if update.Message == nil || !slices.Contains(t.authorizedUsers, update.Message.From.UserName) {
			t.logger.Debugw("Ignoring message from unauthorized user", "user", update.Message.From.UserName)
			continue
		}

		if t.session == nil {
			// There is no active scan session, and the user probably wants one
			t.newScanSession(update)
			continue
		} else if t.session.UserName == update.Message.From.UserName {
			// There is an active scan session we own
			t.handleScanSession(update)
			continue
		} else if t.session.UserName != update.Message.From.UserName {
			t.logAndReportErrorToUser(update, "Scanner busy", nil)
			continue
		}
	}
}

// newScanSession creates a new scan session for the user
func (t *TG) newScanSession(update tgbotapi.Update) {
	if update.Message.Text != createScanSession {
		t.handleUnknownCommand(update.Message.Chat.ID)
		return
	}
	scanSession, err := scansession.Init(
		*t.scanners,
		t.scannerOverride,
		t.tmpDir,
		t.finalDir,
		t.logger,
	)
	scanSession.SetUser(update.Message.From.UserName, update.Message.Chat.ID)
	if err != nil {
		t.logAndReportErrorToUser(update, "Unable to create scan session", err)
		return
	}

	t.session = scanSession

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You have the scanner.")
	msg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(scanPage),
			tgbotapi.NewKeyboardButton(cancel),
		),
	)
	t.logger.Debugw("Created new scan session ",
		"user", update.Message.From.UserName,
		"session", t.session)
	t.bot.Send(msg)
}

func (t *TG) handleScanSession(update tgbotapi.Update) {
	switch update.Message.Text {
	case scanPage:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID,
			"Scanning page, please wait...")
		_, err := t.bot.Send(msg)
		if err != nil {
			t.logAndReportErrorToUser(update, "Unable to send message", err)
			return
		}

		err = t.session.Scan()
		if err != nil {
			t.logAndReportErrorToUser(update, "Unable to scan page", err)
			return
		}
		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("Scanned page %d.", t.session.NumScanned()))
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
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Finishing scan...")
		t.bot.Send(msg)

		fileName, err := t.session.WriteFinal()
		if err != nil {
			t.logAndReportErrorToUser(update, "Unable to write final scan", err)
			return
		}

		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			fmt.Sprintf("Scan finished. Wrote file `%s`.", fileName))
		t.bot.Send(msg)
		t.session = nil
		t.handleUnknownCommand(update.Message.Chat.ID)
	case cancel:
		t.session.Cancel()
		t.session = nil
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Scan cancelled.")
		t.bot.Send(msg)
		t.handleUnknownCommand(update.Message.Chat.ID)
	default:
		t.handleUnknownCommand(update.Message.Chat.ID)
	}
}

func (t *TG) handleUnknownCommand(chatID int64) {
	msg := tgbotapi.NewMessage(chatID,
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
