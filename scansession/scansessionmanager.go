package scansession

type Manager struct {
	tmpDir       string
	finalDir     string
	scanSessions map[string]*ScanSession
}

func NewManager(tmpDir, finalDir string) *Manager {
	return &Manager{
		tmpDir:       tmpDir,
		finalDir:     finalDir,
		scanSessions: make(map[string]*ScanSession),
	}
}

func (s *Manager) TmpDir() string {
	return s.tmpDir
}

func (s *Manager) FinalDir() string {
	return s.finalDir
}

// Make a scansession
func (s *Manager) ScanSession(userName string, chatID int64) *ScanSession {
	if _, ok := s.scanSessions[userName]; ok {
		return s.scanSessions[userName]
	} else {
		s.scanSessions[userName] = NewScanSession(userName, chatID, s.tmpDir, s.finalDir)
		return s.scanSessions[userName]
	}
}

// Remove a scansession
func (s *Manager) RemoveScanSession(userName string) {
	delete(s.scanSessions, userName)
}
