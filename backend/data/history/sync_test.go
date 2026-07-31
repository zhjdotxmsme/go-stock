package history

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSyncParams_Valid(t *testing.T) {
	err := validateSyncParams("sh600519", "day", "2024-01-01", "2024-06-30")
	assert.NoError(t, err)

	err = validateSyncParams("sz000001", "week", "2024-01-01", "2024-06-30")
	assert.NoError(t, err)

	err = validateSyncParams("sh600519", "month", "2024-01-01", "2024-06-30")
	assert.NoError(t, err)
}

func TestValidateSyncParams_EmptyStockCode(t *testing.T) {
	err := validateSyncParams("", "day", "2024-01-01", "2024-06-30")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stock code cannot be empty")
}

func TestValidateSyncParams_InvalidPeriod(t *testing.T) {
	err := validateSyncParams("sh600519", "year", "2024-01-01", "2024-06-30")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid period")

	err = validateSyncParams("sh600519", "", "2024-01-01", "2024-06-30")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "period cannot be empty")
}

func TestValidateSyncParams_EmptyDates(t *testing.T) {
	err := validateSyncParams("sh600519", "day", "", "2024-06-30")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start date cannot be empty")

	err = validateSyncParams("sh600519", "day", "2024-01-01", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "end date cannot be empty")
}

func TestValidateSyncParams_InvalidDateFormat(t *testing.T) {
	err := validateSyncParams("sh600519", "day", "01-01-2024", "2024-06-30")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid start date format")

	err = validateSyncParams("sh600519", "day", "2024-01-01", "06-30-2024")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid end date format")
}

func TestValidateSyncParams_StartAfterEnd(t *testing.T) {
	err := validateSyncParams("sh600519", "day", "2024-06-30", "2024-01-01")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start date cannot be after end date")
}

func TestEstimateExpectedCount(t *testing.T) {
	// day: 2024-01-01 ~ 2024-01-07 is 7 days -> 5 weekdays
	assert.Equal(t, 5, estimateExpectedCount("day", "2024-01-01", "2024-01-07"))
	// week: 7 days -> about 2 weekly bars
	assert.Equal(t, 2, estimateExpectedCount("week", "2024-01-01", "2024-01-07"))
	// month: Jan 2024 ~ Mar 2024 -> 3 monthly bars
	assert.Equal(t, 3, estimateExpectedCount("month", "2024-01-01", "2024-03-31"))
	// invalid range -> 0
	assert.Equal(t, 0, estimateExpectedCount("day", "2024-06-30", "2024-01-01"))
	assert.Equal(t, 0, estimateExpectedCount("day", "bad", "2024-01-01"))
}
