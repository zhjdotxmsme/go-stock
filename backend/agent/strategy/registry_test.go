package strategy

import (
	"testing"
)

func TestRegistryNotEmpty(t *testing.T) {
	strategies := GetAll()
	if len(strategies) == 0 {
		t.Error("Expected at least 1 strategy in registry, got 0")
	}
}

func TestRequiredFields(t *testing.T) {
	strategies := GetAll()
	for i, s := range strategies {
		if s.Code == "" {
			t.Errorf("Strategy[%d] has empty Code", i)
		}
		if s.Name == "" {
			t.Errorf("Strategy[%d] has empty Name", i)
		}
		if s.Prompt == "" {
			t.Errorf("Strategy[%d] has empty Prompt", i)
		}
		if s.Category == "" {
			t.Errorf("Strategy[%d] has empty Category", i)
		}
	}
}

func TestNoDuplicateCodes(t *testing.T) {
	strategies := GetAll()
	codes := make(map[string]bool)
	for _, s := range strategies {
		if codes[s.Code] {
			t.Errorf("Duplicate code found: %s", s.Code)
		}
		codes[s.Code] = true
	}
}

func TestGetByCode(t *testing.T) {
	s := GetByCode("moving_average")
	if s == nil {
		t.Error("GetByCode(\"moving_average\") returned nil, expected strategy")
	}

	nonExistent := GetByCode("non_existent")
	if nonExistent != nil {
		t.Error("GetByCode(\"non_existent\") returned non-nil, expected nil")
	}
}

func TestGetAllSorted(t *testing.T) {
	strategies := GetAll()
	for i := 1; i < len(strategies); i++ {
		if strategies[i-1].Code > strategies[i].Code {
			t.Errorf("Strategies not sorted by Code: %s > %s at index %d", strategies[i-1].Code, strategies[i].Code, i)
		}
	}
}

func TestCategories(t *testing.T) {
	validCategories := map[string]bool{
		"technical":   true,
		"fundamental": true,
		"sentiment":   true,
		"event":       true,
	}
	strategies := GetAll()
	for i, s := range strategies {
		if !validCategories[s.Category] {
			t.Errorf("Strategy[%d] has invalid Category: %s (valid: technical, fundamental, sentiment, event)", i, s.Category)
		}
	}
}

func TestDataNeeds(t *testing.T) {
	validDataNeeds := map[string]bool{
		"kline":       true,
		"news":        true,
		"fundamental": true,
		"sentiment":   true,
	}
	strategies := GetAll()
	for i, s := range strategies {
		for j, dn := range s.DataNeeds {
			if !validDataNeeds[dn] {
				t.Errorf("Strategy[%d].DataNeeds[%d] has invalid value: %s (valid: kline, news, fundamental, sentiment)", i, j, dn)
			}
		}
	}
}
