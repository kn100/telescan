package scanner

import (
	"context"
	"fmt"

	"github.com/brutella/dnssd"
	"github.com/stapelberg/airscan"
	"go.uber.org/zap"
)

type Manager struct {
	logger          *zap.SugaredLogger
	scanners        []*Scanner
	scannerOverride string
	scannerSource   string
	isStarted       bool
}

func NewManager(logger *zap.SugaredLogger, scannerOverride string, scannerSource string) *Manager {
	return &Manager{
		logger:          logger,
		scannerOverride: scannerOverride,
		scannerSource:   scannerSource,
	}
}

func (s *Manager) Start() {
	if s.isStarted {
		s.logger.Infof("Scanner Manager already started, ignoring")
		return
	}
	s.isStarted = true
	addFn := func(srv dnssd.BrowseEntry) {
		for _, n := range s.scanners {
			if n.DNSSDBrowseEntry.Host == srv.Host {
				n.UpdateState(ScannerStateIdle)
				s.logger.Infof("Found scanner that we saw before %s, updated state to idle", srv.Host)

				return
			}
		}
		s.logger.Infof("Found new scanner %s, adding to list of scanners", srv.Host)
		s.scanners = append(s.scanners, Init(&srv, s.logger, s.scannerSource))
	}

	rmvFn := func(srv dnssd.BrowseEntry) {
		for _, n := range s.scanners {
			if n.DNSSDBrowseEntry.Host == srv.Host {
				// We've seen the scanner before, so just set its state to not connected.
				n.UpdateState(ScannerNotConnected)
				s.logger.Infof("Lost scanner %s, updated state to not connected", srv.Host)

				break
			}
		}
	}
	go func() {
		if err := dnssd.LookupType(context.Background(), airscan.ServiceName, addFn, rmvFn); err != nil &&
			err != context.Canceled &&
			err != context.DeadlineExceeded {
			s.logger.Panicw("dnssd.LookupType error", "error", err.Error())
		}
	}()
	s.logger.Infof("Started Scanner Manager")

}

// function to get a specific scanner that has a matching human name
func (s *Manager) GetScanner() (*Scanner, error) {
	if !s.isStarted {
		return nil, fmt.Errorf("scanner manager not started")
	}

	idleScanners := s.idleScanners()

	if len(idleScanners) == 0 {
		return nil, fmt.Errorf("no suitable scanners found")
	}

	if s.scannerOverride == "" {
		return s.scanners[0], nil
	} else {
		for _, observedScanner := range idleScanners {
			if observedScanner.DeviceName() == s.scannerOverride {
				return observedScanner, nil
			}
		}
	}

	return nil, fmt.Errorf("no scanner found with name %s", s.scannerOverride)
}

func (s *Manager) idleScanners() []*Scanner {
	var idleScanners []*Scanner
	for _, sc := range s.scanners {
		if sc.IsIdle() {
			idleScanners = append(idleScanners, sc)
		}
	}
	s.logger.Infow("Getting idle scanners",
		"total_scanner_count", len(s.scanners),
		"idle_scanner_count", len(idleScanners))
	return idleScanners
}
