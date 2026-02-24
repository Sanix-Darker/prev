package serena

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAvailable(t *testing.T) {
	// In most CI/test environments, Serena won't be installed.
	// This test just verifies the function doesn't panic and returns a bool.
	result := IsAvailable()
	assert.IsType(t, false, result)
}

func TestGracefulFallback(t *testing.T) {
	// Reset the once for this test
	// When mode is "auto" and Serena is not installed, NewClient should return nil, nil
	client, err := NewClient("auto")
	if IsAvailable() {
		// If Serena IS available, we get a client - close it
		if client != nil {
			client.Close()
		}
		return
	}
	assert.Nil(t, client)
	assert.NoError(t, err)
}

func TestRequiredMode(t *testing.T) {
	if IsAvailable() {
		t.Skip("Serena is available, can't test required mode failure")
	}

	// Reset the once
	client, err := NewClient("on")
	assert.Nil(t, client)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "serena is required")
}

func TestOffMode(t *testing.T) {
	client, err := NewClient("off")
	assert.Nil(t, client)
	assert.NoError(t, err)
}
