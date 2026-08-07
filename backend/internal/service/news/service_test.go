package news

import (
	"context"
	"testing"

	"go-stock/backend/internal/domain/market"
	"go-stock/backend/internal/port/repository"
)

// stubRepo 嵌入接口:未覆盖的方法被调用时会 panic。
type stubRepo struct {
	repository.TelegraphRepository
	gotSource string
	gotLimit  int
	list      []market.Telegraph
	err       error
}

func (s *stubRepo) GetTelegraphList(ctx context.Context, source string, limit int) ([]market.Telegraph, error) {
	s.gotSource = source
	s.gotLimit = limit
	return s.list, s.err
}

func TestGetTelegraphList(t *testing.T) {
	ctx := context.Background()

	t.Run("透传source并用默认50条上限", func(t *testing.T) {
		repo := &stubRepo{list: []market.Telegraph{{Title: "快讯"}}}
		svc := NewService(repo)
		got, err := svc.GetTelegraphList(ctx, "财联社电报")
		if err != nil {
			t.Fatal(err)
		}
		if repo.gotSource != "财联社电报" {
			t.Errorf("source=%q", repo.gotSource)
		}
		if repo.gotLimit != 50 {
			t.Errorf("limit=%d, want 50", repo.gotLimit)
		}
		if len(got) != 1 || got[0].Title != "快讯" {
			t.Errorf("got=%+v", got)
		}
	})

	t.Run("空source查全部", func(t *testing.T) {
		repo := &stubRepo{}
		svc := NewService(repo)
		if _, err := svc.GetTelegraphList(ctx, ""); err != nil {
			t.Fatal(err)
		}
		if repo.gotSource != "" {
			t.Errorf("source=%q", repo.gotSource)
		}
	})
}
