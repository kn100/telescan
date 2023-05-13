package scanner

import (
	"bytes"
	"fmt"
	"io"
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
	source           string
	logger           *zap.SugaredLogger
}

func Init(srv *dnssd.BrowseEntry, logger *zap.SugaredLogger, source string) *Scanner {
	return &Scanner{
		DNSSDBrowseEntry: srv,
		State:            ScannerStateIdle,
		source:           source,
		logger:           logger,
	}
}

func (s *Scanner) Scan() ([]byte, error) {
	if s.State != ScannerStateIdle {
		return nil, fmt.Errorf(s.stateMsg())
	}

	s.UpdateState(ScannerStateBusy)

	cl := airscan.NewClientForService(s.DNSSDBrowseEntry)

	scan, err := cl.Scan(s.scanSettings())
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

func (s *Scanner) scanSettings() *airscan.ScanSettings {
	settings := preset.GrayscaleA4ADF()
	settings.ColorMode = "RGB24"
	settings.InputSource = s.source
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
