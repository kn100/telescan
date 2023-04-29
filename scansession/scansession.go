package scansession

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tjgq/sane"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"gopkg.in/gographics/imagick.v3/imagick"
)

type ScanSession struct {
	// The user doing the scanning
	ID              string
	UserName        string
	Filename        string
	FilesInScan     []string
	ScanStartTime   time.Time
	ScanLastUpdated time.Time
	scanner         string
	tmpDir          string
	finalDir        string
	logger          *zap.SugaredLogger
}

func Init(username, scannerName, tmpDir, finalDir string, logger *zap.SugaredLogger) (*ScanSession, error) {
	// Firstly, list devices
	scanners := listDevices()
	if len(scanners) == 0 {
		return nil, errors.New("no scanners found")
	}
	logger.Debugf("Found scanners: %s", scanners)
	scanSession := ScanSession{}
	if scannerName != "" {
		if slices.Contains(scanners, scannerName) {
			scanSession.scanner = scannerName
		} else {
			return nil, errors.New("manually specified scanner not found")
		}
	} else {
		scanSession.scanner = scanners[0]
	}
	scanSession.ID = uuid.New().String()
	scanSession.UserName = username
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
	logger.Debugf("Starting scan session %s for user %s", scanSession.ID, scanSession.UserName)
	return &scanSession, nil
}

func (s *ScanSession) scannerAvailable() bool {
	scanners := listDevices()
	if len(scanners) == 0 {
		return false
	}
	if slices.Contains(scanners, s.scanner) {
		return true
	}
	return false

}

func listDevices() []string {
	devs, _ := sane.Devices()
	var names []string
	for _, d := range devs {
		names = append(names, d.Name)
	}
	return names
}

func (s *ScanSession) NumberOfPagesScanned() int {
	return len(s.FilesInScan)
}

func (s *ScanSession) WriteFinal() (string, error) {
	imagick.Initialize()
	defer imagick.Terminate()
	mw := imagick.NewMagickWand()
	mw.SetFormat("pdf")
	for _, f := range s.FilesInScan {
		s.logger.Debugf("Adding %s to PDF", f)
		err := mw.ReadImage(f)
		if err != nil {
			return "", err
		}
	}
	s.logger.Debugf("Writing PDF to %s", filepath.Join(s.finalDir, s.Filename))
	err := mw.WriteImages(filepath.Join(s.finalDir, s.Filename), true)
	if err != nil {
		return "", err
	}
	s.Cancel()
	return s.Filename, nil
}

func (s *ScanSession) Cancel() error {
	return os.RemoveAll(s.TmpPath())
}

func (s *ScanSession) TmpPath() string {
	return filepath.Join(s.tmpDir, s.UserName, s.ID)
}

func (s *ScanSession) Scan() error {
	if !s.scannerAvailable() {
		return errors.New("scanner not available")
	}
	c, err := sane.Open(s.scanner)
	if err != nil {
		return err
	}
	defer c.Close()

	// Create directory for this scan session
	err = os.MkdirAll(s.TmpPath(), 0755)
	if err != nil {
		return err
	}

	// Create file for this scan
	fileName := fmt.Sprintf("%d.png", len(s.FilesInScan))
	pathForThisScan := filepath.Join(s.TmpPath(), fileName)
	f, err := os.Create(pathForThisScan)
	if err != nil {
		panic(err)
	}
	s.logger.Debugf("Created empty file %s", pathForThisScan)

	defer f.Close()

	img, err := c.ReadImage()
	if err != nil {
		panic(err)
	}
	s.logger.Debugf("Read image from scanner", s.scanner)

	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
	s.logger.Debugf("Wrote file", pathForThisScan)
	s.FilesInScan = append(s.FilesInScan, pathForThisScan)
	s.ScanLastUpdated = time.Now()
	s.logger.Debugf("Updated scan session %s last updated time to %s", s.ID, s.ScanLastUpdated)
	return nil
}
