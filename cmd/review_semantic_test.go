package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSemanticBehaviorID_StableAcrossWordOrder(t *testing.T) {
	a := "Missing nil check before request dereference"
	b := "request dereference before missing nil check"

	assert.Equal(t, semanticBehaviorID(a), semanticBehaviorID(b))
}

func TestSemanticMessageScore_BoostsSharedSymbolAndKeywords(t *testing.T) {
	score := semanticMessageScore(
		"ProcessOrder should reject nil payload before dereference",
		"ProcessOrder",
		"Nil payload dereference risk remains in ProcessOrder",
		"ProcessOrder",
	)
	assert.GreaterOrEqual(t, score, 40)
}
