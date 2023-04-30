package main

import (
	"context"
	"os"
	"strings"

	"github.com/brutella/dnssd"
	"github.com/kn100/telescan/tg"
	"github.com/stapelberg/airscan"
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

	var scanners []*dnssd.BrowseEntry
	addFn := func(srv dnssd.BrowseEntry) {
		sugar.Infow("Service discovered", "service", humanDeviceName(srv))
		scanners = append(scanners, &srv)
	}

	rmvFn := func(srv dnssd.BrowseEntry) {
		sugar.Infow("Service %q went away", humanDeviceName(srv))
		// remove from names
		for i, n := range scanners {
			if n.Host == srv.Host {
				scanners = append(scanners[:i], scanners[i+1:]...)
				break
			}
		}
	}
	go func() {
		if err := dnssd.LookupType(context.Background(), airscan.ServiceName, addFn, rmvFn); err != nil &&
			err != context.Canceled &&
			err != context.DeadlineExceeded {
			sugar.Panicw("dnssd.LookupType error", "error", err.Error())
		}
	}()

	tgbot := tg.Init(
		&scanners,
		strings.Split(env("AUTHORIZED_USERS", ""), ","),
		env("TELEGRAM_API_KEY", ""),
		env("TMP_DIR", "/tmp"),
		env("FINAL_DIR", "/final"),
		env("SCANNER_OVERRIDE", ""),
		sugar)
	tgbot.Start()
}

func humanDeviceName(srv dnssd.BrowseEntry) string {
	if ty := srv.Text["ty"]; ty != "" {
		return ty
	}

	// miekg/dns escapes characters in DNS labels: as per RFC1034 and
	// RFC1035, labels do not actually permit whitespace. The purpose of
	// escaping originally appears to be to use these labels in a DNS
	// master file, but for our UI, backslashes look just wrong:
	return strings.ReplaceAll(srv.Name, "\\", "")
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
