package config

import (
	"fmt"
	"io"
	"os"

	"github.com/mitchellh/go-homedir"
	printers "github.com/sanix-darker/prev/internal/printers"
	"github.com/spf13/viper"
)

const (
	HomePath              = "$HOME"
	ConfigDirPath         = HomePath + "/.config/prev"
	ConfigFilePah         = ConfigDirPath + "/config.yml"
	ConversationCachePath = HomePath + "/.prev_cache"
)

// Config contains the entire cli dependencies
type Config struct {
	Version                    string
	Viper                      viper.Viper
	ConfigDirPath              string
	ConfigFilePah              string
	ConfigCachePath            string
	ConfigCachePathFileHistory string
	Debug                      bool
	MaxKeyPoints               int8
	MaxCharactersPerKeyPoints  int16
	ExplainItOrNot             bool
	Spin                       printers.ISpinner
	Printers                   printers.IPrinters

	//io Writers useful for testing
	InReader  io.Reader
	OutWriter io.Writer
	ErrWriter io.Writer
}

// NewDefaultConfig creates a new default config
func NewDefaultConfig() Config {
	conf := Config{
		Spin:                       printers.NewSpinner(),
		Printers:                   printers.NewPrinters(),
		ConfigDirPath:              ".config/prev",
		ConfigFilePah:              "config.yml",
		ConfigCachePath:            ".prev_cache",
		ConfigCachePathFileHistory: "history",
		Debug:                      false,
		MaxKeyPoints:               3,
		MaxCharactersPerKeyPoints:  100,
		ExplainItOrNot:             false, // either we want prev to add in the prompt to explain it or not
		InReader:                   os.Stdin,
		OutWriter:                  os.Stdout,
		ErrWriter:                  os.Stderr,
	}

	conf.Viper = setupViper(conf)
	return conf
}

func setupViper(conf Config) viper.Viper {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	dir, err := GetConfigDirPath(conf)
	if err != nil {
		return viper.Viper{}
	}

	v.AddConfigPath(dir)
	err = v.ReadInConfig()
	if err != nil {
		return viper.Viper{}
	}

	return *v
}

// GetConfigFilePath get the store file path from config
func GetConfigFilePath(conf Config) (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("failed to read home directory: %s", err)
	}

	path := fmt.Sprintf("%s/%s/%s", home, conf.ConfigDirPath, conf.ConfigFilePah)
	return path, nil
}

// GetHistoryFilePath get the history path for prev
func GetHistoryFilePath(conf Config) (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("failed to read home directory: %s", err)
	}

	path := fmt.Sprintf("%s/%s/%s", home, conf.ConfigCachePath, conf.ConfigCachePathFileHistory)
	return path, nil
}

// GetCacheDirPath returns the path of the prev caches
func GetCacheDirPath(conf Config) (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("failed to read home directory: %s", err)
	}

	dir := fmt.Sprintf("%s/%s", home, conf.ConfigCachePath)
	return dir, nil
}

// GetConfigDirPath returns the path of the prev folder
func GetConfigDirPath(conf Config) (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("failed to read home directory: %s", err)
	}

	dir := fmt.Sprintf("%s/%s", home, conf.ConfigDirPath)
	return dir, nil
}
