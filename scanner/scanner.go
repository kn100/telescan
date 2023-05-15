package scanner

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/brutella/dnssd"
	"github.com/stapelberg/airscan"
	"github.com/stapelberg/airscan/preset"
	"go.uber.org/zap"
)

// Define enum that enumerates the scanner state
type ScannerState int

const (
	ScannerStateIdle ScannerState = iota
	ScannerStateBusy
	ScannerNotConnected
)

type Scanner struct {
	DNSSDBrowseEntry *dnssd.BrowseEntry
	State            ScannerState
	logger           *zap.SugaredLogger
}

func Init(srv *dnssd.BrowseEntry, logger *zap.SugaredLogger) *Scanner {
	return &Scanner{
		DNSSDBrowseEntry: srv,
		State:            ScannerStateIdle,
		logger:           logger,
	}
}

func (s *Scanner) Scan() ([]byte, error) {
	if s.State != ScannerStateIdle {
		return nil, fmt.Errorf(s.stateMsg())
	}

	s.UpdateState(ScannerStateBusy)

	cl := airscan.NewClientForService(s.DNSSDBrowseEntry)
	transport := cl.HTTPClient.(*http.Client).Transport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	scannerCapabilities, err := cl.ScannerCapabilities()
	if err != nil {
		s.UpdateState(ScannerStateIdle)
		return nil, err
	}

	ss := scanSettings()
	s.logger.Debugw("Scanner capabilities", "capabilities", scannerCapabilities)
	if scannerCapabilities.Adf != nil {
		s.logger.Infoln("ADF is available on selected scanner, so using it.")
		ss.InputSource = "Feeder"
	}

	scan, err := cl.Scan(ss)
	if err != nil {
		s.UpdateState(ScannerStateIdle)
		return nil, err
	}

	defer scan.Close()

	var f bytes.Buffer
	for scan.ScanPage() {
		if _, err := io.Copy(&f, scan.CurrentPage()); err != nil {
			s.UpdateState(ScannerStateIdle)
			return nil, err
		}
	}

	s.UpdateState(ScannerStateIdle)

	return f.Bytes(), nil
}

func (s *Scanner) GetState() ScannerState {
	return s.State
}

func (s *Scanner) IsIdle() bool {
	return s.State == ScannerStateIdle
}

func (s *Scanner) UpdateState(state ScannerState) {
	s.State = state
}

func (s *Scanner) DeviceName() string {
	if ty := s.DNSSDBrowseEntry.Text["ty"]; ty != "" {
		return ty
	}

	return strings.ReplaceAll(s.DNSSDBrowseEntry.Name, "\\", "")
}

func scanSettings() *airscan.ScanSettings {
	settings := preset.GrayscaleA4ADF()
	settings.ColorMode = "RGB24"
	settings.InputSource = "Platen"
	settings.DocumentFormat = "image/jpeg"
	return settings
}

func (s *Scanner) stateMsg() string {
	switch s.State {
	case ScannerStateIdle:
		return fmt.Sprintf("Scanner %s is idle", s.DeviceName())
	case ScannerStateBusy:
		return fmt.Sprintf("Scanner %s is busy", s.DeviceName())
	case ScannerNotConnected:
		return fmt.Sprintf("Scanner %s is not connected", s.DeviceName())
	}
	return fmt.Sprintf("Scanner %s is in an unknown state", s.DeviceName())
}
