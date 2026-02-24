package common

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestGetArgByKey(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("name", "default", "test flag")

	value := GetArgByKey("name", flags, false, func() error { return nil })
	assert.Equal(t, "default", value)
}

func TestGetArgByKey_MissingNonStrict(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

	// Non-strict mode should not panic on missing flag
	value := GetArgByKey("missing", flags, false, func() error { return nil })
	assert.Empty(t, value)
}

func TestExtractPaths_File(t *testing.T) {
	// Use the fixtures directory which we know exists
	paths := ExtractPaths("../../fixtures/test_diff1.py", func() error { return nil })
	assert.Contains(t, paths, "../../fixtures/test_diff1.py")
}

func TestLogInfo(t *testing.T) {
	called := false
	LogInfo("test message", func() {
		called = true
	})
	assert.True(t, called)
}

func TestLogInfo_NilCallback(t *testing.T) {
	// Should not panic with nil callback
	LogInfo("test message", nil)
}
