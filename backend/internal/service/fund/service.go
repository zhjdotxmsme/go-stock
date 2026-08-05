// Package fund 基金服务
// 该层只依赖 port 接口,不直接引用 data/db。
// 本切片承载自选基金域:关注/取关的文案编排;基金基本信息缺失时的
// 爬取通过 CrawlFundBasicFunc 注入,避免反向依赖 data 层。
package fund

import (
	"context"

	"go-stock/backend/internal/domain/fund"
	"go-stock/backend/internal/port/repository"
)

// CrawlFundBasicFunc 爬取基金基本信息的函数类型;
// 返回 (nil, nil) 或 error 表示获取失败。
type CrawlFundBasicFunc func(fundCode string) (*fund.FundBasic, error)

// Service 基金服务
type Service struct {
	repo    repository.FundRepository
	crawlFn CrawlFundBasicFunc
}

// NewService 创建基金服务;crawlFn 可为 nil(FollowFund 时按爬取失败处理)。
func NewService(repo repository.FundRepository, crawlFn CrawlFundBasicFunc) *Service {
	return &Service{repo: repo, crawlFn: crawlFn}
}

// FollowFund 关注基金;文案与原 data 层逐字一致。
func (s *Service) FollowFund(ctx context.Context, fundCode string) string {
	basic, _ := s.repo.GetFundBasicByCode(ctx, fundCode)
	if basic == nil || basic.Code == "" || basic.Company == "" {
		var crawled *fund.FundBasic
		var err error
		if s.crawlFn != nil {
			crawled, err = s.crawlFn(fundCode)
		}
		if err != nil || crawled == nil {
			if basic == nil || basic.Code == "" {
				return "基金信息不存在或获取失败"
			}
		} else {
			basic = crawled
		}
	}
	follow := &fund.FollowedFund{
		Code: fundCode,
		Name: basic.Name,
	}
	if err := s.repo.FirstOrCreateFollowedFund(ctx, follow, basic.Code); err != nil {
		return "关注失败"
	}
	return "关注成功"
}

// UnFollowFund 取消关注基金;文案与原 data 层逐字一致。
func (s *Service) UnFollowFund(ctx context.Context, fundCode string) string {
	existing, _ := s.repo.GetFollowedFundByCode(ctx, fundCode)
	if existing != nil && existing.Code != "" {
		if err := s.repo.DeleteFollowedFund(ctx, existing); err != nil {
			return "取消关注失败"
		}
		return "取消关注成功"
	}
	return "基金信息不存在"
}

// GetFollowedFund 返回全部关注基金(含实时净值补全)。
func (s *Service) GetFollowedFund(ctx context.Context) ([]fund.FollowedFund, error) {
	return s.repo.ListFollowedFunds(ctx)
}

// GetFollowedFundPaged 关注基金分页(含实时净值补全)。
func (s *Service) GetFollowedFundPaged(ctx context.Context, pageIndex, pageSize int, keyword string) (*fund.FollowedFundPagedResult, error) {
	return s.repo.GetFollowedFundsPaged(ctx, pageIndex, pageSize, keyword)
}
