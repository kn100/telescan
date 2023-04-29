package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kn100/telescan/tg"
	"github.com/tjgq/sane"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("Hi")
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	sugar := logger.Sugar()

	if err := sane.Init(); err != nil {
		sugar.Panic(err)
	}
	defer sane.Exit()

	authedUsers := strings.Split(os.Getenv("AUTHORIZED_USERS"), ",")
	sugar.Debug(authedUsers)
	tgbot := tg.Init(
		os.Getenv("TELEGRAM_API_KEY"),
		authedUsers,
		os.Getenv("TMP_DIR"),
		os.Getenv("FINAL_DIR"),
		os.Getenv("SCANNER_OVERRIDE"),
		sugar)
	tgbot.Start()
}
