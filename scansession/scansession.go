package scansession

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brutella/dnssd"
	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"github.com/stapelberg/airscan"
	"github.com/stapelberg/airscan/preset"
	"go.uber.org/zap"
)

type ScanSession struct {
	// The user doing the scanning
	ID              string
	UserName        string
	ChatID          int64
	Filename        string
	FilesInScan     []string
	ScanStartTime   time.Time
	ScanLastUpdated time.Time
	scanner         *dnssd.BrowseEntry
	tmpDir          string
	finalDir        string
	logger          *zap.SugaredLogger
}

func (s *ScanSession) SetUser(userName string, ChatID int64) {
	s.UserName = userName
	s.ChatID = ChatID
}

func Init(scanners []*dnssd.BrowseEntry, scannerName, tmpDir, finalDir string, logger *zap.SugaredLogger) (*ScanSession, error) {
	if len(scanners) == 0 {
		return nil, errors.New("no scanners found, try again later")
	}

	scanSession := ScanSession{}
	if scannerName != "" {
		scanner := locateScanner(scanners, scannerName)
		if scanner != nil {
			scanSession.scanner = scanner
		} else {
			return nil, errors.New("manually specified scanner not found")
		}
	} else {
		scanSession.scanner = scanners[0]
	}

	scanSession.ID = uuid.New().String()
	scanSession.tmpDir = tmpDir
	scanSession.finalDir = finalDir
	scanSession.ScanStartTime = time.Now()
	scanSession.ScanLastUpdated = time.Now()
	scanSession.logger = logger
	scanSession.Filename = fmt.Sprintf(
		"%s-%s.pdf",
		scanSession.UserName,
		scanSession.ScanStartTime.Format("2006-01-02-15-04-05"),
	)

	logger.Infow("Initialized scan session",
		"detected_scanners", scanners,
		"selected_scanner", scanSession.scanner,
		"scannerName", scannerName,
		"tmpDir", tmpDir,
		"finalDir", finalDir)
	return &scanSession, nil
}

func (s *ScanSession) NumScanned() int {
	return len(s.FilesInScan)
}

func (s *ScanSession) WriteFinal() (string, error) {
	for i := 0; i < len(s.FilesInScan); i++ {
		imp, _ := api.Import("form:A4P, pos:c, sc:1 rel", types.POINTS)
		api.ImportImagesFile([]string{s.FilesInScan[i]}, filepath.Join(s.finalDir, s.Filename), imp, nil)
	}

	return s.Filename, nil
}

func (s *ScanSession) Cancel() error {
	return os.RemoveAll(s.TmpPath())
}

func (s *ScanSession) TmpPath() string {
	return filepath.Join(s.tmpDir, s.UserName, s.ID)
}

func (s *ScanSession) Scan() error {

	// Create directory for this scan session
	err := os.MkdirAll(s.TmpPath(), 0755)
	if err != nil {
		return err
	}

	// // Create file for this scan
	fileName := fmt.Sprintf("%d.jpg", len(s.FilesInScan))
	pathForThisScan := filepath.Join(s.TmpPath(), fileName)
	f, err := os.Create(pathForThisScan)
	if err != nil {
		return err
	}
	defer f.Close()
	s.logger.Debugf("Created empty file %s", pathForThisScan)

	cl := airscan.NewClientForService(s.scanner)
	scan, err := cl.Scan(scanSettings())
	if err != nil {
		return err
	}
	defer scan.Close()

	for scan.ScanPage() {
		if _, err := io.Copy(f, scan.CurrentPage()); err != nil {
			return err
		}
	}

	s.FilesInScan = append(s.FilesInScan, pathForThisScan)
	s.ScanLastUpdated = time.Now()
	s.logger.Debugf("Updated scan session %s last updated time to %s", s.ID, s.ScanLastUpdated)
	return nil
}

func scanSettings() *airscan.ScanSettings {
	settings := preset.GrayscaleA4ADF()
	settings.ColorMode = "RGB24"
	settings.InputSource = "Platen"
	settings.DocumentFormat = "image/jpeg"
	return settings
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

func locateScanner(scanners []*dnssd.BrowseEntry, humanName string) *dnssd.BrowseEntry {
	for _, s := range scanners {
		if humanDeviceName(*s) == humanName {
			return s
		}
	}
	return nil
}
