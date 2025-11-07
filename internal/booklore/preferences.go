package booklore

import (
	"sync"

	"go.uber.org/zap"
)

// UserPreferences stores user-specific settings
type UserPreferences struct {
	libraryID  int64
	pathID     int64
	libraryName string
	pathName    string
}

// PreferenceManager manages user preferences in memory
type PreferenceManager struct {
	preferences map[int64]*UserPreferences
	mutex       sync.RWMutex
	logger      *zap.Logger
}

// NewPreferenceManager creates a new preference manager
func NewPreferenceManager(logger *zap.Logger) *PreferenceManager {
	return &PreferenceManager{
		preferences: make(map[int64]*UserPreferences),
		logger:      logger,
	}
}

// GetUserPreference gets user's library preference
func (pm *PreferenceManager) GetUserPreference(userID int64) *UserPreferences {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if pref, exists := pm.preferences[userID]; exists {
		return pref
	}

	// Return empty preference if not set
	return &UserPreferences{}
}

// GetLibraryID returns the library ID
func (up *UserPreferences) GetLibraryID() int64 {
	return up.libraryID
}

// GetPathID returns the path ID
func (up *UserPreferences) GetPathID() int64 {
	return up.pathID
}

// GetLibraryName returns the library name
func (up *UserPreferences) GetLibraryName() string {
	return up.libraryName
}

// GetPathName returns the path name
func (up *UserPreferences) GetPathName() string {
	return up.pathName
}

// HasLibrary returns true if a library is selected
func (up *UserPreferences) HasLibrary() bool {
	return up.libraryID > 0
}

// SetUserPreference sets user's library preference
func (pm *PreferenceManager) SetUserPreference(userID int64, libraryID, pathID int64, libraryName, pathName string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.preferences[userID] = &UserPreferences{
		libraryID:  libraryID,
		pathID:     pathID,
		libraryName: libraryName,
		pathName:    pathName,
	}

	pm.logger.Info("User preference set",
		zap.Int64("user_id", userID),
		zap.Int64("library_id", libraryID),
		zap.Int64("path_id", pathID),
		zap.String("library_name", libraryName),
		zap.String("path_name", pathName))
}

// ClearUserPreference removes user's library preference
func (pm *PreferenceManager) ClearUserPreference(userID int64) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	delete(pm.preferences, userID)
	pm.logger.Info("User preference cleared",
		zap.Int64("user_id", userID))
}