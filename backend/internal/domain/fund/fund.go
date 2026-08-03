package fund

import (
	"gorm.io/gorm"
)

// FollowedFund 自选基金
type FollowedFund struct {
	gorm.Model
	Code string `json:"code" gorm:"index"`
	Name string `json:"name"`

	NetUnitValue     *float64 `json:"netUnitValue"`
	NetUnitValueDate string   `json:"netUnitValueDate"`
	NetEstimatedUnit *float64 `json:"netEstimatedUnit"`
	NetEstimatedTime string   `json:"netEstimatedUnitTime"`
	NetAccumulated   *float64 `json:"netAccumulated"`

	NetEstimatedRate *float64 `json:"netEstimatedRate"`

	NetUnitValuePrev *float64 `json:"netUnitValuePrev"`
	NetActualRate    *float64 `json:"netActualRate"`

	FundBasic FundBasic `json:"fundBasic" gorm:"foreignKey:Code;references:Code"`
}

func (FollowedFund) TableName() string {
	return "followed_fund"
}

// FundBasic 基金基本信息
type FundBasic struct {
	gorm.Model
	Code           string `json:"code" gorm:"index"`
	Name           string `json:"name"`
	FullName       string `json:"fullName"`
	Type           string `json:"type"`
	Establishment  string `json:"establishment"`
	Scale          string `json:"scale"`
	Company        string `json:"company"`
	Manager        string `json:"manager"`
	Rating         string `json:"rating"`
	TrackingTarget string `json:"trackingTarget"`

	NetUnitValue     *float64 `json:"netUnitValue"`
	NetUnitValueDate string   `json:"netUnitValueDate"`
	NetEstimatedUnit *float64 `json:"netEstimatedUnit"`
	NetEstimatedTime string   `json:"netEstimatedUnitTime"`
	NetAccumulated   *float64 `json:"netAccumulated"`

	NetGrowth1   *float64 `json:"netGrowth1"`
	NetGrowth3   *float64 `json:"netGrowth3"`
	NetGrowth6   *float64 `json:"netGrowth6"`
	NetGrowth12  *float64 `json:"netGrowth12"`
	NetGrowth36  *float64 `json:"netGrowth36"`
	NetGrowth60  *float64 `json:"netGrowth60"`
	NetGrowthYTD *float64 `json:"netGrowthYTD"`
	NetGrowthAll *float64 `json:"netGrowthAll"`
}

func (FundBasic) TableName() string {
	return "fund_basic"
}

// FollowedFundPagedResult 自选基金分页结果
type FollowedFundPagedResult struct {
	Items      []FollowedFund `json:"items"`
	Total      int64          `json:"total"`
	PageIndex  int            `json:"pageIndex"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}
