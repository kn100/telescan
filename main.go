package main

import (
	"os"
	"strings"

	"github.com/kn100/telescan/scanner"
	"github.com/kn100/telescan/scansession"
	"github.com/kn100/telescan/tg"
	"go.uber.org/zap"
)

func main() {
	var logger *zap.Logger
	var err error
	debug := env("DEBUG", "false")
	if debug == "true" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	sugar := logger.Sugar()

	scannerManager := scanner.NewManager(sugar, env("SCANNER_OVERRIDE", ""))
	scannerManager.Start()

	scanSessionManager := scansession.NewManager(
		env("TMP_DIR", "/tmp"),
		env("FINAL_DIR", "/final"),
	)

	tgconfig := tg.TGConfig{
		APIKey:         env("TELEGRAM_API_KEY", ""),
		Users:          strings.Split(env("AUTHORIZED_USERS", ""), ","),
		SendScanToChat: env("SEND_SCAN_TO_CHAT", "false") == "true",
	}

	tgbot := tg.Init(
		tgconfig,
		scannerManager,
		scanSessionManager,
		sugar)
	tgbot.Start()
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
