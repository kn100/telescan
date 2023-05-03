package scansession

import (
	"fmt"
	"io/ioutil"
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

func (s *ScanSession) AddImage(imgBytes []byte) {
	s.ScanLastUpdated = time.Now()
	s.filesInScan = append(s.filesInScan, imgBytes)
}

func (s *ScanSession) WriteFinal() (string, error) {
	// Dump all images in s.filesInScan to tmpDir/s.Filename named incrementally
	// Then, use pdfcpu to merge them into a single PDF
	filesOnDisk := make([]string, len(s.filesInScan))
	for i := 0; i < len(s.filesInScan); i++ {
		fileName := fmt.Sprintf("%s-%d.jpg", s.Filename(), i)
		filePath := filepath.Join(s.tmpDir, fileName)
		err := ioutil.WriteFile(filePath, s.filesInScan[i], 0644)
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

	return s.Filename(), err
}

func (s *ScanSession) Filename() string {
	return fmt.Sprintf("%s-%s.pdf", s.userName, s.ScanStartTime.Format("2006-01-02-15-04-05"))
}

func (s *ScanSession) NumImages() int {
	return len(s.filesInScan)
}
