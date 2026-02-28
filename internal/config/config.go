package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	printers "github.com/sanix-darker/prev/internal/printers"
)

const (
	HomePath              = "$HOME"
	ConfigDirPath         = HomePath + "/.config/prev"
	ConfigFilePath        = ConfigDirPath + "/config.yml"
	ConversationCachePath = HomePath + "/.prev_cache"
)

// Config contains the entire cli dependencies
type Config struct {
	Version                    string
	Viper                      *Store
	ConfigDirPath              string
	ConfigFilePath             string
	ConfigCachePath            string
	ConfigCachePathFileHistory string
	Debug                      bool
	MaxKeyPoints               int8
	MaxCharactersPerKeyPoints  int16
	ExplainItOrNot             bool
	Provider                   string
	Model                      string
	Stream                     bool
	Strictness                 string
	ContextLines               int
	MaxBatchTokens             int
	SerenaMode                 string
	Printers                   printers.IPrinters

	//io Writers useful for testing
	InReader  io.Reader
	OutWriter io.Writer
	ErrWriter io.Writer
}

// NewDefaultConfig creates a new default config
func NewDefaultConfig() Config {
	conf := Config{
		Printers:                   printers.NewPrinters(),
		ConfigDirPath:              ".config/prev",
		ConfigFilePath:             "config.yml",
		ConfigCachePath:            ".prev_cache",
		ConfigCachePathFileHistory: "history",
		Debug:                      false,
		MaxKeyPoints:               3,
		MaxCharactersPerKeyPoints:  100,
		ExplainItOrNot:             false,
		Provider:                   "openai",
		Stream:                     true,
		Strictness:                 "normal",
		ContextLines:               10,
		MaxBatchTokens:             80000,
		SerenaMode:                 "auto",
		InReader:                   os.Stdin,
		OutWriter:                  os.Stdout,
		ErrWriter:                  os.Stderr,
	}

	conf.Viper = setupStore(conf)
	return conf
}

func setupStore(conf Config) *Store {
	s := NewStore()

	dir, err := GetConfigDirPath(conf)
	if err != nil {
		return s
	}

	cfgFile := filepath.Join(dir, conf.ConfigFilePath)
	if err := s.LoadYAMLFile(cfgFile); err != nil {
		// Config file not found is OK, we use defaults
		return s
	}

	return s
}

// GetConfigFilePath get the store file path from config
func GetConfigFilePath(conf Config) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to read home directory: %s", err)
	}

	path := fmt.Sprintf("%s/%s/%s", home, conf.ConfigDirPath, conf.ConfigFilePath)
	return path, nil
}

// GetHistoryFilePath get the history path for prev
func GetHistoryFilePath(conf Config) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to read home directory: %s", err)
	}

	path := fmt.Sprintf("%s/%s/%s", home, conf.ConfigCachePath, conf.ConfigCachePathFileHistory)
	return path, nil
}

// GetCacheDirPath returns the path of the prev caches
func GetCacheDirPath(conf Config) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to read home directory: %s", err)
	}

	dir := fmt.Sprintf("%s/%s", home, conf.ConfigCachePath)
	return dir, nil
}

// GetConfigDirPath returns the path of the prev folder
func GetConfigDirPath(conf Config) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to read home directory: %s", err)
	}

	dir := fmt.Sprintf("%s/%s", home, conf.ConfigDirPath)
	return dir, nil
}
