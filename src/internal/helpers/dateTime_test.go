package helpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateDateNow(t *testing.T) {
	mockTime := time.Date(2024, 9, 27, 8, 59, 59, 0, time.UTC)
	SetMockNow(func() time.Time { return mockTime })
	defer ResetMockNow()

	expected := "2024-09-27T08:59:59Z"
	result := GenerateDateNow()

	assert.Equal(t, expected, result)
}

func TestGenerateDateNowMs(t *testing.T) {
	mockTime := time.Date(2024, 9, 27, 8, 59, 59, 123000000, time.UTC)
	SetMockNow(func() time.Time { return mockTime })
	defer ResetMockNow()

	expected := "2024-09-27T08:59:59.123Z"
	result := GenerateDateNowMs()

	assert.Equal(t, expected, result)
}
