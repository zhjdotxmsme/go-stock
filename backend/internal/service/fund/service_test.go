package fund

import (
	"context"
	"errors"
	"testing"

	"go-stock/backend/internal/domain/fund"
	"go-stock/backend/internal/port/repository"
)

// stubRepo 嵌入接口:未覆盖的方法被调用时会 panic,可暴露 service 对 port 的误用。
type stubRepo struct {
	repository.FundRepository
	basic      *fund.FundBasic
	basicErr   error
	existing   *fund.FollowedFund
	created    *fund.FollowedFund
	assignCode string
	createErr  error
	deleted    *fund.FollowedFund
	deleteErr  error
}

func (s *stubRepo) GetFundBasicByCode(ctx context.Context, code string) (*fund.FundBasic, error) {
	return s.basic, s.basicErr
}

func (s *stubRepo) FirstOrCreateFollowedFund(ctx context.Context, follow *fund.FollowedFund, assignCode string) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = follow
	s.assignCode = assignCode
	return nil
}

func (s *stubRepo) GetFollowedFundByCode(ctx context.Context, code string) (*fund.FollowedFund, error) {
	return s.existing, nil
}

func (s *stubRepo) DeleteFollowedFund(ctx context.Context, follow *fund.FollowedFund) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = follow
	return nil
}

func TestFollowFund_BasicComplete_SkipsCrawl(t *testing.T) {
	repo := &stubRepo{basic: &fund.FundBasic{Code: "110022", Name: "易方达消费", Company: "易方达"}}
	crawlCalled := false
	svc := NewService(repo, func(code string) (*fund.FundBasic, error) {
		crawlCalled = true
		return nil, nil
	})
	if got := svc.FollowFund(context.Background(), "110022"); got != "关注成功" {
		t.Errorf("got %q, want %q", got, "关注成功")
	}
	if crawlCalled {
		t.Error("crawlFn should not be called when basic info is complete")
	}
	if repo.created == nil || repo.created.Code != "110022" || repo.created.Name != "易方达消费" {
		t.Errorf("created = %+v", repo.created)
	}
	if repo.assignCode != "110022" {
		t.Errorf("assignCode = %q, want 110022", repo.assignCode)
	}
}

func TestFollowFund_BasicMissing_CrawlSuccess(t *testing.T) {
	repo := &stubRepo{} // basic == nil
	svc := NewService(repo, func(code string) (*fund.FundBasic, error) {
		return &fund.FundBasic{Code: "110022", Name: "易方达消费", Company: "易方达"}, nil
	})
	if got := svc.FollowFund(context.Background(), "110022"); got != "关注成功" {
		t.Errorf("got %q, want %q", got, "关注成功")
	}
	if repo.created == nil || repo.created.Name != "易方达消费" {
		t.Errorf("created = %+v", repo.created)
	}
}

func TestFollowFund_BasicMissing_CrawlFails(t *testing.T) {
	repo := &stubRepo{} // basic == nil
	svc := NewService(repo, func(code string) (*fund.FundBasic, error) {
		return nil, errors.New("network down")
	})
	if got := svc.FollowFund(context.Background(), "110022"); got != "基金信息不存在或获取失败" {
		t.Errorf("got %q, want %q", got, "基金信息不存在或获取失败")
	}
	if repo.created != nil {
		t.Error("FirstOrCreateFollowedFund should not be called")
	}
}

func TestFollowFund_CompanyMissing_CrawlFails_UsesStoredName(t *testing.T) {
	// DB 有记录但 Company 为空,爬取失败时仍用已存 Name 继续关注(与原 data 层一致)
	repo := &stubRepo{basic: &fund.FundBasic{Code: "110022", Name: "易方达消费"}}
	svc := NewService(repo, func(code string) (*fund.FundBasic, error) {
		return nil, nil
	})
	if got := svc.FollowFund(context.Background(), "110022"); got != "关注成功" {
		t.Errorf("got %q, want %q", got, "关注成功")
	}
	if repo.created == nil || repo.created.Name != "易方达消费" {
		t.Errorf("created = %+v", repo.created)
	}
}

func TestFollowFund_CreateError(t *testing.T) {
	repo := &stubRepo{
		basic:     &fund.FundBasic{Code: "110022", Name: "易方达消费", Company: "易方达"},
		createErr: errors.New("db down"),
	}
	svc := NewService(repo, nil)
	if got := svc.FollowFund(context.Background(), "110022"); got != "关注失败" {
		t.Errorf("got %q, want %q", got, "关注失败")
	}
}

func TestUnFollowFund_Exists(t *testing.T) {
	repo := &stubRepo{existing: &fund.FollowedFund{Code: "110022"}}
	svc := NewService(repo, nil)
	if got := svc.UnFollowFund(context.Background(), "110022"); got != "取消关注成功" {
		t.Errorf("got %q, want %q", got, "取消关注成功")
	}
	if repo.deleted == nil || repo.deleted.Code != "110022" {
		t.Errorf("deleted = %+v", repo.deleted)
	}
}

func TestUnFollowFund_DeleteError(t *testing.T) {
	repo := &stubRepo{
		existing:  &fund.FollowedFund{Code: "110022"},
		deleteErr: errors.New("db down"),
	}
	svc := NewService(repo, nil)
	if got := svc.UnFollowFund(context.Background(), "110022"); got != "取消关注失败" {
		t.Errorf("got %q, want %q", got, "取消关注失败")
	}
}

func TestUnFollowFund_NotExists(t *testing.T) {
	svc := NewService(&stubRepo{}, nil)
	if got := svc.UnFollowFund(context.Background(), "110022"); got != "基金信息不存在" {
		t.Errorf("got %q, want %q", got, "基金信息不存在")
	}
}
