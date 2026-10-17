package config

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
)

// ErrUnknownKey is returned when trying to set an unregistered configuration key
var ErrUnknownKey = errors.New("unknown configuration key")

// ErrAPIDisabled is returned when the API is disabled for a user
var ErrAPIDisabled = errors.New("API is disabled for this user")

// ConfigService provides methods to manage user settings
type ConfigService struct {
	store *store.Store
}

// NewConfigService creates a new ConfigService instance
func NewConfigService(store *store.Store) *ConfigService {
	return &ConfigService{store: store}
}

// Get retrieves a single configuration value for a user
func (s *ConfigService) Get(userID, key string) (any, error) {
	logger.Debug("[ConfigService.Get] userID=%s, key=%s", userID, key)

	value, err := s.store.GetSetting(userID, key)

	if err != nil {
		logger.Debug("[ConfigService.Get] Error finding record: %v", err)
		// Return default value if not found
		return GetDefault(key), nil
	}

	if isSensitiveKey(key) {
		logger.Debug("[ConfigService.Get] Found value: %s (type: %T)", maskSensitiveValue(s.parseStringValue(value)), value)
	} else {
		logger.Debug("[ConfigService.Get] Found value: %v (type: %T)", value, value)
	}
	return value, nil
}

// GetString retrieves a string configuration value
func (s *ConfigService) GetString(userID, key string) (string, error) {
	value, err := s.Get(userID, key)
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", nil
	}

	if str, ok := value.(string); ok {
		return str, nil
	}
	return "", nil
}

// GetBool retrieves a boolean configuration value
func (s *ConfigService) GetBool(userID, key string) (bool, error) {
	value, err := s.Get(userID, key)
	if err != nil {
		return false, err
	}
	if value == nil {
		return false, nil
	}

	// Handle different types that JSON might return
	switch v := value.(type) {
	case bool:
		return v, nil
	case float64:
		return v != 0, nil
	case string:
		return v == "true", nil
	}
	return false, nil
}

// GetInt retrieves an integer configuration value for a user
func (s *ConfigService) GetInt(userID, key string) (int, error) {
	value, err := s.Get(userID, key)
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, nil
	}
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, nil
		}
	}
	return 0, nil
}

// Set stores a configuration value for a user
func (s *ConfigService) Set(userID, key string, value any) error {
	// Validate key against registry
	if _, ok := GetConfigMeta(key); !ok {
		return ErrUnknownKey
	}

	return s.store.SetSetting(userID, key, value, IsEncrypted(key))
}

// GetBatch retrieves all configuration values for a user
func (s *ConfigService) GetBatch(userID string) (map[string]any, error) {
	return s.store.GetSettings(userID)
}

// SetBatch stores multiple configuration values for a user atomically
func (s *ConfigService) SetBatch(userID string, settings map[string]any) error {
	for key, value := range settings {
		// Skip unknown keys with warning log
		if _, ok := GetConfigMeta(key); !ok {
			logger.Warn("[ConfigService.SetBatch] unknown key: %s, skipping", key)
			continue
		}
		if err := s.store.SetSetting(userID, key, value, IsEncrypted(key)); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a configuration value for a user
func (s *ConfigService) Delete(userID, key string) error {
	return s.store.DeleteSetting(userID, key)
}

// maskSensitiveValue returns a masked version of sensitive values for safe logging
func maskSensitiveValue(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}

// isSensitiveKey checks if a key contains sensitive data that should be masked in logs
func isSensitiveKey(key string) bool {
	return IsEncrypted(key) || key == "api.token"
}

// ValidateTokenAndGetUser validates an API token and returns the user ID
func (s *ConfigService) ValidateTokenAndGetUser(token string) (string, error) {
	logger.Debug("[ValidateTokenAndGetUser] validating token: %s", maskSensitiveValue(token))

	userID, err := s.store.ValidateAPIToken(token)
	if err != nil && errors.Is(err, store.ErrAPIDisabled) {
		return "", ErrAPIDisabled
	}
	if err != nil || userID == "" {
		logger.Debug("[ValidateTokenAndGetUser] no matching token found: %v", err)
		return "", nil
	}

	storedToken, _ := s.GetString(userID, "api.token")

	// Use constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(storedToken), []byte(token)) != 1 {
		logger.Debug("[ValidateTokenAndGetUser] token mismatch")
		return "", nil
	}

	// Check if API is enabled for this user
	enabled, err := s.GetBool(userID, "api.enabled")
	if err != nil {
		logger.Debug("[ValidateTokenAndGetUser] error checking API enabled: %v", err)
		return "", err
	}
	if !enabled {
		logger.Debug("[ValidateTokenAndGetUser] API disabled for user: %s", userID)
		return "", ErrAPIDisabled
	}

	logger.Debug("[ValidateTokenAndGetUser] token validated for user: %s", userID)
	return userID, nil
}

// parseStringValue extracts a string from various value types
func (s *ConfigService) parseStringValue(value any) string {
	if value == nil {
		return ""
	}

	if str, ok := value.(string); ok {
		return str
	}
	bytes, err := json.Marshal(value)
	if err == nil {
		var str string
		if json.Unmarshal(bytes, &str) == nil {
			return str
		}
	}
	return ""
}
