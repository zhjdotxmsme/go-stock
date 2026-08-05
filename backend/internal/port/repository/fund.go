package repository

import (
	"context"

	"go-stock/backend/internal/domain/fund"
)

// FundRepository abstracts persistence for fund-related entities.
// Implementations live in backend/internal/adapter/repository/sqlite.
type FundRepository interface {
	// GetFundBasicByCode 按代码查询基金基本信息;不存在时返回 (nil, nil)。
	GetFundBasicByCode(ctx context.Context, code string) (*fund.FundBasic, error)
	// FirstOrCreateFollowedFund 按代码查不到时创建关注记录;创建时 Code 赋值为 assignCode
	// (与原 data 层 FirstOrCreate(attrs) 语义一致)。
	FirstOrCreateFollowedFund(ctx context.Context, follow *fund.FollowedFund, assignCode string) error
	// GetFollowedFundByCode 按代码查询关注记录;不存在时返回 (nil, nil)。
	GetFollowedFundByCode(ctx context.Context, code string) (*fund.FollowedFund, error)
	DeleteFollowedFund(ctx context.Context, follow *fund.FollowedFund) error

	// ListFollowedFunds 返回全部关注基金(含实时净值补全,实现侧委托 data 层)。
	ListFollowedFunds(ctx context.Context) ([]fund.FollowedFund, error)
	// GetFollowedFundsPaged 关注基金分页(含实时净值补全,实现侧委托 data 层)。
	GetFollowedFundsPaged(ctx context.Context, pageIndex, pageSize int, keyword string) (*fund.FollowedFundPagedResult, error)
}
