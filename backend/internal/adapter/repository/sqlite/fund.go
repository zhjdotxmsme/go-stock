package sqlite

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/internal/domain/fund"
	"go-stock/backend/logger"
)

// FundRepository implements repository.FundRepository.
//
// Follow/Unfollow primitives are pure DB and written directly with GORM;
// the list/paged reads embed realtime net-value crawling and rate
// computation, so they delegate to the existing data-layer API with
// explicit mapping (same pattern as the trading-record delegation).
type FundRepository struct{}

// NewFundRepository creates a new FundRepository.
func NewFundRepository() *FundRepository {
	return &FundRepository{}
}

// ---------------------------------------------------------------------------
// data <-> domain mapping (explicit, no reflection)
// ---------------------------------------------------------------------------

// FundBasicToDomain maps a data-layer GORM model to the domain model.
func FundBasicToDomain(m *data.FundBasic) *fund.FundBasic {
	if m == nil {
		return nil
	}
	return &fund.FundBasic{
		Model:            m.Model,
		Code:             m.Code,
		Name:             m.Name,
		FullName:         m.FullName,
		Type:             m.Type,
		Establishment:    m.Establishment,
		Scale:            m.Scale,
		Company:          m.Company,
		Manager:          m.Manager,
		Rating:           m.Rating,
		TrackingTarget:   m.TrackingTarget,
		NetUnitValue:     m.NetUnitValue,
		NetUnitValueDate: m.NetUnitValueDate,
		NetEstimatedUnit: m.NetEstimatedUnit,
		NetEstimatedTime: m.NetEstimatedTime,
		NetAccumulated:   m.NetAccumulated,
		NetGrowth1:       m.NetGrowth1,
		NetGrowth3:       m.NetGrowth3,
		NetGrowth6:       m.NetGrowth6,
		NetGrowth12:      m.NetGrowth12,
		NetGrowth36:      m.NetGrowth36,
		NetGrowth60:      m.NetGrowth60,
		NetGrowthYTD:     m.NetGrowthYTD,
		NetGrowthAll:     m.NetGrowthAll,
	}
}

// FundBasicFromDomain maps a domain model to the data-layer GORM model.
func FundBasicFromDomain(f *fund.FundBasic) *data.FundBasic {
	if f == nil {
		return nil
	}
	return &data.FundBasic{
		Model:            f.Model,
		Code:             f.Code,
		Name:             f.Name,
		FullName:         f.FullName,
		Type:             f.Type,
		Establishment:    f.Establishment,
		Scale:            f.Scale,
		Company:          f.Company,
		Manager:          f.Manager,
		Rating:           f.Rating,
		TrackingTarget:   f.TrackingTarget,
		NetUnitValue:     f.NetUnitValue,
		NetUnitValueDate: f.NetUnitValueDate,
		NetEstimatedUnit: f.NetEstimatedUnit,
		NetEstimatedTime: f.NetEstimatedTime,
		NetAccumulated:   f.NetAccumulated,
		NetGrowth1:       f.NetGrowth1,
		NetGrowth3:       f.NetGrowth3,
		NetGrowth6:       f.NetGrowth6,
		NetGrowth12:      f.NetGrowth12,
		NetGrowth36:      f.NetGrowth36,
		NetGrowth60:      f.NetGrowth60,
		NetGrowthYTD:     f.NetGrowthYTD,
		NetGrowthAll:     f.NetGrowthAll,
	}
}

// FollowedFundToDomain maps a data-layer GORM model to the domain model.
func FollowedFundToDomain(m *data.FollowedFund) *fund.FollowedFund {
	if m == nil {
		return nil
	}
	return &fund.FollowedFund{
		Model:            m.Model,
		Code:             m.Code,
		Name:             m.Name,
		NetUnitValue:     m.NetUnitValue,
		NetUnitValueDate: m.NetUnitValueDate,
		NetEstimatedUnit: m.NetEstimatedUnit,
		NetEstimatedTime: m.NetEstimatedTime,
		NetAccumulated:   m.NetAccumulated,
		NetEstimatedRate: m.NetEstimatedRate,
		NetUnitValuePrev: m.NetUnitValuePrev,
		NetActualRate:    m.NetActualRate,
		FundBasic:        *FundBasicToDomain(&m.FundBasic),
	}
}

// FollowedFundFromDomain maps a domain model to the data-layer GORM model.
func FollowedFundFromDomain(f *fund.FollowedFund) *data.FollowedFund {
	if f == nil {
		return nil
	}
	return &data.FollowedFund{
		Model:            f.Model,
		Code:             f.Code,
		Name:             f.Name,
		NetUnitValue:     f.NetUnitValue,
		NetUnitValueDate: f.NetUnitValueDate,
		NetEstimatedUnit: f.NetEstimatedUnit,
		NetEstimatedTime: f.NetEstimatedTime,
		NetAccumulated:   f.NetAccumulated,
		NetEstimatedRate: f.NetEstimatedRate,
		NetUnitValuePrev: f.NetUnitValuePrev,
		NetActualRate:    f.NetActualRate,
		FundBasic:        *FundBasicFromDomain(&f.FundBasic),
	}
}

// FollowedFundPagedResultFromDomain maps a domain page result back to the
// data-layer type (used by handlers that keep data-typed signatures).
func FollowedFundPagedResultFromDomain(p *fund.FollowedFundPagedResult) *data.FollowedFundPagedResult {
	if p == nil {
		return &data.FollowedFundPagedResult{}
	}
	items := make([]data.FollowedFund, 0, len(p.Items))
	for i := range p.Items {
		items = append(items, *FollowedFundFromDomain(&p.Items[i]))
	}
	return &data.FollowedFundPagedResult{
		Items:      items,
		TotalCount: p.Total,
		PageIndex:  p.PageIndex,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
}

// ---------------------------------------------------------------------------
// Follow / unfollow primitives (pure DB, GORM)
// ---------------------------------------------------------------------------

func (r *FundRepository) GetFundBasicByCode(ctx context.Context, code string) (*fund.FundBasic, error) {
	var basic data.FundBasic
	err := db.Dao.Where("code=?", code).First(&basic).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.SugaredLogger.Errorf("查询基金基本信息失败: %s", err.Error())
		return nil, err
	}
	return FundBasicToDomain(&basic), nil
}

func (r *FundRepository) FirstOrCreateFollowedFund(ctx context.Context, follow *fund.FollowedFund, assignCode string) error {
	d := FollowedFundFromDomain(follow)
	err := db.Dao.Model(d).Where("code = ?", d.Code).FirstOrCreate(d, "code", assignCode).Error
	if err != nil {
		return err
	}
	follow.Model = d.Model
	return nil
}

func (r *FundRepository) GetFollowedFundByCode(ctx context.Context, code string) (*fund.FollowedFund, error) {
	var followed data.FollowedFund
	err := db.Dao.Where("code=?", code).First(&followed).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logger.SugaredLogger.Errorf("查询关注基金失败: %s", err.Error())
		return nil, err
	}
	return FollowedFundToDomain(&followed), nil
}

func (r *FundRepository) DeleteFollowedFund(ctx context.Context, follow *fund.FollowedFund) error {
	d := FollowedFundFromDomain(follow)
	return db.Dao.Model(d).Delete(d).Error
}

// ---------------------------------------------------------------------------
// Followed-fund reads (delegate to data API: realtime crawl + rate assembly)
// ---------------------------------------------------------------------------

func (r *FundRepository) ListFollowedFunds(ctx context.Context) ([]fund.FollowedFund, error) {
	funds := data.NewFundApi().GetFollowedFund()
	result := make([]fund.FollowedFund, 0, len(funds))
	for i := range funds {
		result = append(result, *FollowedFundToDomain(&funds[i]))
	}
	return result, nil
}

func (r *FundRepository) GetFollowedFundsPaged(ctx context.Context, pageIndex, pageSize int, keyword string) (*fund.FollowedFundPagedResult, error) {
	paged := data.NewFundApi().GetFollowedFundPaged(pageIndex, pageSize, keyword)
	if paged == nil {
		return &fund.FollowedFundPagedResult{}, nil
	}
	items := make([]fund.FollowedFund, 0, len(paged.Items))
	for i := range paged.Items {
		items = append(items, *FollowedFundToDomain(&paged.Items[i]))
	}
	return &fund.FollowedFundPagedResult{
		Items:      items,
		Total:      paged.TotalCount,
		PageIndex:  paged.PageIndex,
		PageSize:   paged.PageSize,
		TotalPages: paged.TotalPages,
	}, nil
}
