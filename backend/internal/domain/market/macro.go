package market

import "gorm.io/gorm"

// PPIResult PPI API结果
type PPIResult struct {
	Month         string  `json:"month"`
	Date          string  `json:"date"`
	PPIBaseYoY    string  `json:"ppi_base_yoy"`
	PPIBaseYoYFloat float64 `json:"-"`
}

// PMIResult PMI API结果
type PMIResult struct {
	Month        string  `json:"month"`
	Date         string  `json:"date"`
	ManPMI       string  `json:"man_pmi"`
	ManPMIFloat  float64 `json:"-"`
	NonManPmiBusi string `json:"non_man_pmi_busi"`
	NonManPmiBusiFloat float64 `json:"-"`
}
type GDP struct {
	gorm.Model
	Month      string  `json:"MONTH"`
	GDPYoY     float64 `json:"GDP_YOY"`
	GDPMom     float64 `json:"GDP_MOM"`
	GDPAccuVal float64 `json:"GDP_ACCU_VAL"`
	Date       string  `json:"date" gorm:"index"`
}

func (GDP) TableName() string {
	return "cn_gdp"
}

// CPI CPI数据
type CPI struct {
	gorm.Model
	Month           string  `json:"MONTH"`
	CPIBaseYoY      float64 `json:"CPI_BASE_YOY"`
	CPINBaseYoY     float64 `json:"CPI_NBASE_YOY"`
	CPINBaseMom     float64 `json:"CPI_NBASE_MOM"`
	CPINBaseAccu    float64 `json:"CPI_NBASE_ACCU"`
	CPITBaseYoY     float64 `json:"CPI_TBASE_YOY"`
	FoodCPITBaseYoY float64 `json:"FOOD_CPI_TBASE_YOY"`
	Date            string  `json:"date" gorm:"index"`
}

func (CPI) TableName() string {
	return "cn_cpi"
}

// PPI PPI数据
type PPI struct {
	gorm.Model
	Month            string  `json:"MONTH"`
	PPIBaseYoY       float64 `json:"PPI_BASE_YOY"`
	PPINBaseYoY      float64 `json:"PPI_NBASE_YOY"`
	PPIRMYoY         float64 `json:"PPI_RM_YOY"`
	PPIGoodsBaseYoY  float64 `json:"PPI_GOODS_BASE_YOY"`
	PPIEngryBaseYoY  float64 `json:"PPI_ENGRY_BASE_YOY"`
	PPIConsBaseYoY   float64 `json:"PPI_CONS_BASE_YOY"`
	PPIProcIndBaseYoY float64 `json:"PPI_PROC_IND_BASE_YOY"`
	PPIBaseMoM       float64 `json:"PPI_BASE_MOM"`
	Date             string  `json:"date" gorm:"index"`
}

func (PPI) TableName() string {
	return "cn_ppi"
}

// PMI PMI数据
type PMI struct {
	gorm.Model
	Month         string  `json:"MONTH"`
	ManPMI        float64 `json:"MAN_PMI"`
	ManPMIProd    float64 `json:"MAN_PMI_PROD"`
	ManPMINewOrd  float64 `json:"MAN_PMI_NEW_ORD"`
	ManPMIExport  float64 `json:"MAN_PMI_EXPORT"`
	ManPMIHoPend  float64 `json:"MAN_PMI_HO_PEND"`
	ManPMIPur     float64 `json:"MAN_PMI_PUR"`
	ManPMIImp     float64 `json:"MAN_PMI_IMP"`
	ManPMIPurPrice float64 `json:"MAN_PMI_PUR_PRICE"`
	ManPMIExitPrice float64 `json:"MAN_PMI_EXIT_PRICE"`
	ManPMIRawMat  float64 `json:"MAN_PMI_RAW_MAT"`
	ManPMIEmploy  float64 `json:"MAN_PMI_EMPLOY"`
	ManPMISupTime float64 `json:"MAN_PMI_SUP_TIME"`
	ManPMIProdBus float64 `json:"MAN_PMI_PROD_BUS"`
	NonManPmiBusi float64 `json:"NON_MAN_PMI_BUSI"`
	NonManPmiNewOrd float64 `json:"NON_MAN_PMI_NEW_ORD"`
	Date          string  `json:"date" gorm:"index"`
}

func (PMI) TableName() string {
	return "cn_pmi"
}

// GDPResult GDP API结果
type GDPResult struct {
	Month          string  `json:"month"`
	Date           string  `json:"date"`
	GDPYoY         string  `json:"gdp_yoy"`
	GDPMom         string  `json:"gdp_mom"`
	GDPAccuVal     string  `json:"gdp_accu_val"`
	GDPYoYFloat    float64 `json:"-"`
	GDPMomFloat    float64 `json:"-"`
	GDPAccuValFloat float64 `json:"-"`
}

// CPIResult CPI API结果
type CPIResult struct {
	Month            string `json:"month"`
	Date             string `json:"date"`
	CPINBaseYoY      string `json:"cpi_nbase_yoy"`
	CPINBaseYoYFloat float64 `json:"-"`
}

// GDPResp GDP API响应
type GDPResp struct {
	Data []GDPResult `json:"data"`
}

// CPIResp CPI API响应
type CPIResp struct {
	Data []CPIResult `json:"data"`
}

// PPIResp PPI API响应
type PPIResp struct {
	Data []PPIResult `json:"data"`
}

// PMIResp PMI API响应
type PMIResp struct {
	Data []PMIResult `json:"data"`
}
