package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultConfig(t *testing.T) {
	conf := NewDefaultConfig()

	assert.False(t, conf.Debug)
	assert.Equal(t, int8(3), conf.MaxKeyPoints)
	assert.Equal(t, int16(100), conf.MaxCharactersPerKeyPoints)
	assert.False(t, conf.ExplainItOrNot)
	assert.Equal(t, "openai", conf.Provider)
	assert.True(t, conf.Stream)
	assert.NotNil(t, conf.Viper)
	assert.NotNil(t, conf.Printers)
	assert.NotNil(t, conf.InReader)
	assert.NotNil(t, conf.OutWriter)
	assert.NotNil(t, conf.ErrWriter)
}

func TestGetConfigFilePath(t *testing.T) {
	conf := NewDefaultConfig()
	path, err := GetConfigFilePath(conf)
	require.NoError(t, err)
	assert.Contains(t, path, ".config/prev")
	assert.Contains(t, path, "config.yml")
}

func TestGetConfigDirPath(t *testing.T) {
	conf := NewDefaultConfig()
	dir, err := GetConfigDirPath(conf)
	require.NoError(t, err)
	assert.Contains(t, dir, ".config/prev")
}

func TestSetupStore_NoConfigFile(t *testing.T) {
	conf := Config{
		ConfigDirPath:  "/nonexistent/path",
		ConfigFilePath: "config.yml",
	}
	v := setupStore(conf)
	assert.NotNil(t, v)
}
