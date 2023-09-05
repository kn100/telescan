package scansession

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type ScanSession struct {
	userName        string
	chatID          int64
	ScanStartTime   time.Time
	ScanLastUpdated time.Time
	tmpDir          string
	finalDir        string
	filesInScan     [][]byte
	wroteFinal      bool
}

func NewScanSession(userName string, chatID int64, tmpDir, finalDir string) *ScanSession {
	return &ScanSession{
		userName:        userName,
		chatID:          chatID,
		ScanStartTime:   time.Now(),
		ScanLastUpdated: time.Now(),
		tmpDir:          tmpDir,
		finalDir:        finalDir,
	}
}

func (s *ScanSession) AddImages(imgBytes []bytes.Buffer) {
	s.ScanLastUpdated = time.Now()
	for _, img := range imgBytes {
		s.filesInScan = append(s.filesInScan, img.Bytes())
	}
}

func (s *ScanSession) WriteFinal() (string, error) {
	// TODO: Interlace the images if the scanner is ADF simplex
	filesOnDisk := make([]string, len(s.filesInScan))
	for i := 0; i < len(s.filesInScan); i++ {
		fileName := fmt.Sprintf("%s-%d.jpg", s.Filename(), i)
		filePath := filepath.Join(s.tmpDir, fileName)
		err := os.WriteFile(filePath, s.filesInScan[i], 0644)
		if err != nil {
			return "", err
		}
		filesOnDisk[i] = filePath
	}

	imp, _ := api.Import("form:A4P, pos:c, sc:1 rel", types.POINTS)
	err := api.ImportImagesFile(filesOnDisk, filepath.Join(s.finalDir, s.Filename()), imp, nil)

	for _, f := range filesOnDisk {
		err := os.Remove(f)
		if err != nil {
			return "", err
		}
	}

	s.wroteFinal = true

	return s.Filename(), err
}

func (s *ScanSession) Filename() string {
	return fmt.Sprintf("%s-%s.pdf", s.userName, s.ScanStartTime.Format("2006-01-02-15-04-05"))
}

func (s *ScanSession) FullPathToScan() string {
	return filepath.Join(s.finalDir, s.Filename())
}

func (s *ScanSession) NumImages() int {
	return len(s.filesInScan)
}

func (s *ScanSession) IsWritten() bool {
	return s.wroteFinal
}
