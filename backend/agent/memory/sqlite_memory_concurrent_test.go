package memory

import (
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestConcurrentNewSQLiteMemory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	roles := []string{"fundamental", "technical", "sentiment", "news", "policy", "hotmoney", "lockup"}
	for _, role := range roles {
		wg.Add(1)
		go func(role string) {
			defer wg.Done()
			m, err := NewSQLiteMemory(db, role)
			if err != nil {
				t.Errorf("NewSQLiteMemory(%s) error: %v", role, err)
				return
			}
			_ = m
		}(role)
	}
	wg.Wait()
}
