package booklore

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// PreferenceManager manages user preferences with persistent storage
type PreferenceManager struct {
	preferences map[int64]*UserPreferences
	mutex       sync.RWMutex
	logger      *zap.Logger
	storagePath string
}

// NewPreferenceManager creates a new preference manager
func NewPreferenceManager(logger *zap.Logger, storagePath string) *PreferenceManager {
	pm := &PreferenceManager{
		preferences: make(map[int64]*UserPreferences),
		logger:      logger,
		storagePath: storagePath,
	}

	// Load existing preferences from file
	pm.loadPreferences()

	return pm
}

// loadPreferences loads preferences from JSON file
func (pm *PreferenceManager) loadPreferences() {
	if pm.storagePath == "" {
		pm.logger.Info("No storage path provided, using in-memory storage only")
		return
	}

	// Ensure directory exists
	dir := filepath.Dir(pm.storagePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		pm.logger.Error("Failed to create preferences directory",
			zap.String("path", dir),
			zap.Error(err))
		return
	}

	// Check if file exists
	if _, err := os.Stat(pm.storagePath); os.IsNotExist(err) {
		pm.logger.Info("Preferences file does not exist, starting fresh",
			zap.String("path", pm.storagePath))
		return
	}

	// Read and parse file
	data, err := os.ReadFile(pm.storagePath)
	if err != nil {
		pm.logger.Error("Failed to read preferences file",
			zap.String("path", pm.storagePath),
			zap.Error(err))
		return
	}

	var storedPrefs map[int64]*UserPreferences
	if err := json.Unmarshal(data, &storedPrefs); err != nil {
		pm.logger.Error("Failed to parse preferences file",
			zap.String("path", pm.storagePath),
			zap.Error(err))
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.preferences = storedPrefs
	pm.logger.Info("Loaded user preferences from file",
		zap.String("path", pm.storagePath),
		zap.Int("user_count", len(pm.preferences)))
}

// savePreferences saves preferences to JSON file
func (pm *PreferenceManager) savePreferences() {
	if pm.storagePath == "" {
		return
	}

	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	data, err := json.MarshalIndent(pm.preferences, "", "  ")
	if err != nil {
		pm.logger.Error("Failed to marshal preferences",
			zap.Error(err))
		return
	}

	if err := os.WriteFile(pm.storagePath, data, 0644); err != nil {
		pm.logger.Error("Failed to save preferences file",
			zap.String("path", pm.storagePath),
			zap.Error(err))
		return
	}

	pm.logger.Debug("Saved user preferences to file",
		zap.String("path", pm.storagePath),
		zap.Int("user_count", len(pm.preferences)))
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

	// Save to persistent storage
	go pm.savePreferences()
}

// ClearUserPreference removes user's library preference
func (pm *PreferenceManager) ClearUserPreference(userID int64) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	delete(pm.preferences, userID)
	pm.logger.Info("User preference cleared",
		zap.Int64("user_id", userID))

	// Save to persistent storage
	go pm.savePreferences()
}