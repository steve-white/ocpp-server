package helpers

import (
	"time"
)

var mockNow func() time.Time

func init() {
	mockNow = time.Now
}

func SetMockNow(mock func() time.Time) {
	mockNow = mock
}

func ResetMockNow() {
	mockNow = time.Now
}

func Now() time.Time {
	return mockNow()
}
