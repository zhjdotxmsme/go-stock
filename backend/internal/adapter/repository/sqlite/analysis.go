package sqlite

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/internal/domain/analysis"
	"go-stock/backend/models"
)

// AnalysisRepository implements repository.AnalysisRepository.
// 查询条件/分页默认值/排序/文案语义逐项复刻原 data 层实现。
type AnalysisRepository struct{}

// NewAnalysisRepository creates a new AnalysisRepository.
func NewAnalysisRepository() *AnalysisRepository {
	return &AnalysisRepository{}
}

// ---------------------------------------------------------------------------
// AI 分析结果（ai_response_result）
// ---------------------------------------------------------------------------

func (r *AnalysisRepository) SaveAIResponseResult(ctx context.Context, item *analysis.AIResponseResult) error {
	return db.Dao.Create(item).Error
}

func (r *AnalysisRepository) GetLatestAIResponseResult(ctx context.Context, stockCode string) (*analysis.AIResponseResult, error) {
	var item analysis.AIResponseResult
	err := db.Dao.Where("stock_code = ?", stockCode).Order("id desc").Limit(1).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *AnalysisRepository) GetAIResponseResultList(ctx context.Context, query analysis.AIResponseResultQuery) (*analysis.AIResponseResultPageData, error) {
	var list []analysis.AIResponseResult
	var total int64

	q := db.Dao.Model(&analysis.AIResponseResult{})

	// 与原 data 层一致：ChatId 用 Where，后续条件用 Or 串联；
	// query.StockName 存在但原实现未参与筛选，保持原样。
	if query.ChatId != "" {
		q = q.Where("chat_id LIKE ?", "%"+query.ChatId+"%")
	}
	if query.ModelName != "" {
		q = q.Or("model_name LIKE ?", "%"+query.ModelName+"%")
	}
	if query.StockCode != "" {
		q = q.Or("stock_code LIKE ?", "%"+query.StockCode+"%")
	}
	if query.Question != "" {
		q = q.Or("question LIKE ?", "%"+query.Question+"%")
	}
	if query.StartDate != "" && query.EndDate != "" {
		startDate := parseFlexDate(query.StartDate)
		endDate := parseFlexDate(query.EndDate)
		q = q.Where("created_at BETWEEN ? AND ?", beginOfDay(startDate), endOfDay(endDate))
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	if err := q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}

	return &analysis.AIResponseResultPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages(total, pageSize),
	}, nil
}

func (r *AnalysisRepository) DeleteAIResponseResult(ctx context.Context, id uint) error {
	return db.Dao.Where("id = ?", id).Delete(&analysis.AIResponseResult{}).Error
}

func (r *AnalysisRepository) BatchDeleteAIResponseResult(ctx context.Context, ids []uint) error {
	return db.Dao.Where("id IN ?", ids).Delete(&analysis.AIResponseResult{}).Error
}

// ---------------------------------------------------------------------------
// AI 推荐股票（ai_recommend_stocks）
// ---------------------------------------------------------------------------

func (r *AnalysisRepository) GetAiRecommendStocksList(ctx context.Context, query analysis.AiRecommendStocksQuery) (*analysis.AiRecommendStocksPageData, error) {
	var list []analysis.AiRecommendStocks
	var total int64

	q := db.Dao.Model(&analysis.AiRecommendStocks{})

	// 关键词：stock_code/stock_name/bk_name/model_name 取第一个非空，四列 OR 模糊匹配
	keyword := query.StockCode
	if keyword == "" {
		keyword = query.StockName
	}
	if keyword == "" {
		keyword = query.BkName
	}
	if keyword == "" {
		keyword = query.ModelName
	}
	if keyword != "" {
		q = q.Where("(stock_code LIKE ? OR stock_name LIKE ? OR bk_name LIKE ? OR model_name LIKE ?)",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 日期范围（data_time）；无日期且无关键词时默认只查今天
	if query.StartDate != "" && query.EndDate != "" {
		startDate := parseFlexDate(query.StartDate)
		endDate := parseFlexDate(query.EndDate)
		q = q.Where("data_time BETWEEN ? AND ?", beginOfDay(startDate), endOfDay(endDate))
	} else if query.StartDate == "" && query.EndDate == "" && keyword == "" {
		q = q.Where("data_time BETWEEN ? AND ?", beginOfDay(time.Now()), endOfDay(time.Now()))
	} else if query.StartDate != "" && query.EndDate == "" {
		startDate := parseFlexDate(query.StartDate)
		q = q.Where("data_time BETWEEN ? AND ?", beginOfDay(startDate), endOfDay(startDate))
	}

	if query.EnableAlert != nil {
		q = q.Where("enable_alert = ?", *query.EnableAlert)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10 // 原实现此处无 >100 上限，保持原样
	}

	offset := (page - 1) * pageSize
	if err := q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}

	return &analysis.AiRecommendStocksPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages(total, pageSize),
	}, nil
}

func (r *AnalysisRepository) DeleteAiRecommendStocks(ctx context.Context, id uint) error {
	return db.Dao.Where("id = ?", id).Delete(&analysis.AiRecommendStocks{}).Error
}

func (r *AnalysisRepository) UpdateAiRecommendStocksAlert(ctx context.Context, id uint, enableAlert bool) error {
	return db.Dao.Model(&analysis.AiRecommendStocks{}).Where("id = ?", id).Update("enable_alert", enableAlert).Error
}

func (r *AnalysisRepository) ListAllAiRecommendStocks(ctx context.Context) ([]analysis.AiRecommendStocks, error) {
	var all []analysis.AiRecommendStocks
	if err := db.Dao.Model(&analysis.AiRecommendStocks{}).Find(&all).Error; err != nil {
		return nil, err
	}
	return all, nil
}

// ---------------------------------------------------------------------------
// 自定义策略（custom_strategies）
// ---------------------------------------------------------------------------

func (r *AnalysisRepository) GetCustomStrategyList(ctx context.Context, query analysis.CustomStrategyQuery) (*analysis.CustomStrategyPageData, error) {
	var list []analysis.CustomStrategy
	var total int64

	q := db.Dao.Model(&analysis.CustomStrategy{})
	if query.Name != "" {
		q = q.Where("name LIKE ?", "%"+query.Name+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	if err := q.Offset(offset).Limit(pageSize).Order("sort_order ASC, created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}

	return &analysis.CustomStrategyPageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages(total, pageSize),
	}, nil
}

func (r *AnalysisRepository) ListAllCustomStrategies(ctx context.Context) ([]analysis.CustomStrategy, error) {
	var list []analysis.CustomStrategy
	err := db.Dao.Model(&analysis.CustomStrategy{}).Order("sort_order ASC, created_at DESC").Find(&list).Error
	return list, err
}

func (r *AnalysisRepository) GetCustomStrategyByID(ctx context.Context, id uint) (*analysis.CustomStrategy, error) {
	var item analysis.CustomStrategy
	err := db.Dao.Model(&analysis.CustomStrategy{}).Where("id=?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *AnalysisRepository) CreateCustomStrategy(ctx context.Context, strategy *analysis.CustomStrategy) error {
	// 原实现只写入四列
	return db.Dao.Model(&analysis.CustomStrategy{}).Create(&analysis.CustomStrategy{
		Name:        strategy.Name,
		Query:       strategy.Query,
		Description: strategy.Description,
		SortOrder:   strategy.SortOrder,
	}).Error
}

func (r *AnalysisRepository) UpdateCustomStrategy(ctx context.Context, strategy *analysis.CustomStrategy) error {
	return db.Dao.Model(&analysis.CustomStrategy{}).Where("id=?", strategy.ID).Updates(map[string]any{
		"name":        strategy.Name,
		"query":       strategy.Query,
		"description": strategy.Description,
		"sort_order":  strategy.SortOrder,
	}).Error
}

func (r *AnalysisRepository) DeleteCustomStrategy(ctx context.Context, id uint) error {
	return db.Dao.Model(&analysis.CustomStrategy{}).Where("id=?", id).Delete(&analysis.CustomStrategy{}).Error
}

// ---------------------------------------------------------------------------
// 提示词模板（prompt_templates）
// ---------------------------------------------------------------------------

func (r *AnalysisRepository) GetPromptTemplates(ctx context.Context, name, promptType string) ([]analysis.PromptTemplate, error) {
	var result []analysis.PromptTemplate
	q := db.Dao.Model(&analysis.PromptTemplate{})
	switch {
	case name != "" && promptType != "":
		q = q.Where("name=? and type=?", name, promptType)
	case name != "":
		q = q.Where("name=?", name)
	case promptType != "":
		q = q.Where("type=?", promptType)
	}
	err := q.Find(&result).Error
	return result, err
}

func (r *AnalysisRepository) GetPromptTemplateList(ctx context.Context, query analysis.PromptTemplateQuery) (*analysis.PromptTemplatePageData, error) {
	var list []analysis.PromptTemplate
	var total int64

	q := db.Dao.Model(&analysis.PromptTemplate{})
	if query.Name != "" {
		q = q.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Type != "" {
		q = q.Where("type LIKE ?", "%"+query.Type+"%")
	}
	if query.Content != "" {
		q = q.Where("content LIKE ?", "%"+query.Content+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	if err := q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}

	return &analysis.PromptTemplatePageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages(total, pageSize),
	}, nil
}

func (r *AnalysisRepository) GetPromptTemplateByID(ctx context.Context, id uint) (*analysis.PromptTemplate, error) {
	var item analysis.PromptTemplate
	err := db.Dao.Model(&analysis.PromptTemplate{}).Where("id=?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *AnalysisRepository) CreatePromptTemplate(ctx context.Context, template *analysis.PromptTemplate) error {
	// 原实现只写入三列（不写 role_key）
	return db.Dao.Model(&analysis.PromptTemplate{}).Create(&analysis.PromptTemplate{
		Content: template.Content,
		Name:    template.Name,
		Type:    template.Type,
	}).Error
}

func (r *AnalysisRepository) UpdatePromptTemplate(ctx context.Context, template *analysis.PromptTemplate) error {
	return db.Dao.Model(&analysis.PromptTemplate{}).Where("id=?", template.ID).Updates(template).Error
}

func (r *AnalysisRepository) DeletePromptTemplate(ctx context.Context, id uint) error {
	return db.Dao.Model(&analysis.PromptTemplate{}).Where("id=?", id).Delete(&analysis.PromptTemplate{}).Error
}

func (r *AnalysisRepository) ListMultiAgentPrompts(ctx context.Context) ([]analysis.PromptTemplate, error) {
	var list []analysis.PromptTemplate
	err := db.Dao.Model(&analysis.PromptTemplate{}).
		Where("type IN ?", []string{"multi_agent", "single_agent"}).
		Find(&list).Error
	return list, err
}

func (r *AnalysisRepository) UpsertPromptByRoleKey(ctx context.Context, roleKey, name, content, ptype string) error {
	var pt analysis.PromptTemplate
	result := db.Dao.Model(&analysis.PromptTemplate{}).Where("role_key = ?", roleKey).First(&pt)
	if result.Error != nil {
		return db.Dao.Create(&analysis.PromptTemplate{
			Name:    name,
			Content: content,
			Type:    ptype,
			RoleKey: roleKey,
		}).Error
	}
	return db.Dao.Model(&analysis.PromptTemplate{}).Where("role_key = ?", roleKey).Updates(map[string]interface{}{
		"name":    name,
		"content": content,
	}).Error
}

// ---------------------------------------------------------------------------
// 共享助手（复刻原 data 层日期处理：T/Z 替换、两种布局、当日边界）
// ---------------------------------------------------------------------------

var dateTZReplacer = strings.NewReplacer("T", " ", "Z", "")

// parseFlexDate 先按 "2006-01-02 15:04:05" 解析，失败退化为 "2006-01-02"。
func parseFlexDate(s string) time.Time {
	s = dateTZReplacer.Replace(s)
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// beginOfDay / endOfDay 与 lancet datetime.BeginOfDay/EndOfDay 等价。
func beginOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

func totalPages(total int64, pageSize int) int {
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}

// ---------------------------------------------------------------------------
// data/models <-> domain 显式映射（不反射）
// ---------------------------------------------------------------------------

// AIResponseResultQueryToDomain maps a models query to the domain query.
func AIResponseResultQueryToDomain(q models.AIResponseResultQuery) *analysis.AIResponseResultQuery {
	return &analysis.AIResponseResultQuery{
		Page:      q.Page,
		PageSize:  q.PageSize,
		ChatId:    q.ChatId,
		ModelName: q.ModelName,
		StockCode: q.StockCode,
		StockName: q.StockName,
		Question:  q.Question,
		StartDate: q.StartDate,
		EndDate:   q.EndDate,
	}
}

// AiRecommendStocksQueryToDomain maps a models query to the domain query.
func AiRecommendStocksQueryToDomain(q models.AiRecommendStocksQuery) *analysis.AiRecommendStocksQuery {
	return &analysis.AiRecommendStocksQuery{
		Page:        q.Page,
		PageSize:    q.PageSize,
		ModelName:   q.ModelName,
		StockCode:   q.StockCode,
		StockName:   q.StockName,
		BkCode:      q.BkCode,
		BkName:      q.BkName,
		StartDate:   q.StartDate,
		EndDate:     q.EndDate,
		EnableAlert: q.EnableAlert,
	}
}

// CustomStrategyQueryToDomain maps a models query to the domain query.
func CustomStrategyQueryToDomain(q models.CustomStrategyQuery) *analysis.CustomStrategyQuery {
	return &analysis.CustomStrategyQuery{
		Page:     q.Page,
		PageSize: q.PageSize,
		Name:     q.Name,
	}
}

// PromptTemplateQueryToDomain maps a models query to the domain query.
func PromptTemplateQueryToDomain(q models.PromptTemplateQuery) *analysis.PromptTemplateQuery {
	return &analysis.PromptTemplateQuery{
		Page:     q.Page,
		PageSize: q.PageSize,
		Name:     q.Name,
		Type:     q.Type,
		Content:  q.Content,
	}
}

// AIResponseResultFromDomain maps a domain model to the data-layer GORM model.
func AIResponseResultFromDomain(r *analysis.AIResponseResult) *models.AIResponseResult {
	if r == nil {
		return nil
	}
	return &models.AIResponseResult{
		Model:     r.Model,
		ChatId:    r.ChatId,
		ModelName: r.ModelName,
		StockCode: r.StockCode,
		StockName: r.StockName,
		Question:  r.Question,
		Content:   r.Content,
		IsDel:     r.IsDel,
	}
}

// AIResponseResultPageDataFromDomain maps a domain page to the models page.
func AIResponseResultPageDataFromDomain(p *analysis.AIResponseResultPageData) *models.AIResponseResultPageData {
	if p == nil {
		return nil
	}
	out := &models.AIResponseResultPageData{
		Total:      p.Total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
	for i := range p.List {
		out.List = append(out.List, *AIResponseResultFromDomain(&p.List[i]))
	}
	return out
}

// AiRecommendStocksFromDomain maps a domain model to the data-layer GORM model.
func AiRecommendStocksFromDomain(r *analysis.AiRecommendStocks) *models.AiRecommendStocks {
	if r == nil {
		return nil
	}
	return &models.AiRecommendStocks{
		Model:                       r.Model,
		DataTime:                    r.DataTime,
		ModelName:                   r.ModelName,
		Rating:                      r.Rating,
		StockCode:                   r.StockCode,
		StockName:                   r.StockName,
		BkCode:                      r.BkCode,
		BkName:                      r.BkName,
		StockPrice:                  r.StockPrice,
		StockCurrentPrice:           r.StockCurrentPrice,
		StockCurrentPriceTime:       r.StockCurrentPriceTime,
		StockClosePrice:             r.StockClosePrice,
		StockPrePrice:               r.StockPrePrice,
		RecommendReason:             r.RecommendReason,
		RecommendBuyPrice:           r.RecommendBuyPrice,
		RecommendBuyPriceMin:        r.RecommendBuyPriceMin,
		RecommendBuyPriceMax:        r.RecommendBuyPriceMax,
		RecommendStopProfitPrice:    r.RecommendStopProfitPrice,
		RecommendStopProfitPriceMin: r.RecommendStopProfitPriceMin,
		RecommendStopProfitPriceMax: r.RecommendStopProfitPriceMax,
		RecommendStopLossPrice:      r.RecommendStopLossPrice,
		RiskRemarks:                 r.RiskRemarks,
		Remarks:                     r.Remarks,
		EnableAlert:                 r.EnableAlert,
	}
}

// AiRecommendStocksPageDataFromDomain maps a domain page to the models page.
func AiRecommendStocksPageDataFromDomain(p *analysis.AiRecommendStocksPageData) *models.AiRecommendStocksPageData {
	if p == nil {
		return nil
	}
	out := &models.AiRecommendStocksPageData{
		Total:      p.Total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
	for i := range p.List {
		out.List = append(out.List, *AiRecommendStocksFromDomain(&p.List[i]))
	}
	return out
}

// AiRecommendStatsFromDomain maps domain stats to the data-layer stats type
// (handler 对外契约类型为 data.AiRecommendStats)。
func AiRecommendStatsFromDomain(s *analysis.AiRecommendStats) *data.AiRecommendStats {
	if s == nil {
		return nil
	}
	out := &data.AiRecommendStats{}
	for _, m := range s.ByModel {
		out.ByModel = append(out.ByModel, data.ModelStat{
			ModelName: m.ModelName, WinRate: m.WinRate, AvgReturn: m.AvgReturn, Count: m.Count,
		})
	}
	for _, sec := range s.BySector {
		out.BySector = append(out.BySector, data.SectorStat{BkName: sec.BkName, Count: sec.Count})
	}
	for _, d := range s.DailyCount {
		out.DailyCount = append(out.DailyCount, data.DailyCount{Date: d.Date, Count: d.Count})
	}
	return out
}

// CustomStrategyToDomain maps a data-layer model to the domain model.
func CustomStrategyToDomain(s *models.CustomStrategy) *analysis.CustomStrategy {
	if s == nil {
		return nil
	}
	return &analysis.CustomStrategy{
		ID:          s.ID,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		Name:        s.Name,
		Query:       s.Query,
		Description: s.Description,
		SortOrder:   s.SortOrder,
	}
}

// CustomStrategyFromDomain maps a domain model to the data-layer GORM model.
func CustomStrategyFromDomain(s *analysis.CustomStrategy) *models.CustomStrategy {
	if s == nil {
		return nil
	}
	return &models.CustomStrategy{
		ID:          s.ID,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		Name:        s.Name,
		Query:       s.Query,
		Description: s.Description,
		SortOrder:   s.SortOrder,
	}
}

// CustomStrategyPageDataFromDomain maps a domain page to the models page.
func CustomStrategyPageDataFromDomain(p *analysis.CustomStrategyPageData) *models.CustomStrategyPageData {
	if p == nil {
		return nil
	}
	out := &models.CustomStrategyPageData{
		Total:      p.Total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
	for i := range p.List {
		out.List = append(out.List, *CustomStrategyFromDomain(&p.List[i]))
	}
	return out
}

// PromptTemplateToDomain maps a data-layer model to the domain model.
func PromptTemplateToDomain(t *models.PromptTemplate) *analysis.PromptTemplate {
	if t == nil {
		return nil
	}
	return &analysis.PromptTemplate{
		ID:        t.ID,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
		Name:      t.Name,
		Content:   t.Content,
		Type:      t.Type,
		RoleKey:   t.RoleKey,
	}
}

// PromptTemplateFromDomain maps a domain model to the data-layer GORM model.
func PromptTemplateFromDomain(t *analysis.PromptTemplate) *models.PromptTemplate {
	if t == nil {
		return nil
	}
	return &models.PromptTemplate{
		ID:        t.ID,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
		Name:      t.Name,
		Content:   t.Content,
		Type:      t.Type,
		RoleKey:   t.RoleKey,
	}
}

// PromptTemplateListFromDomain maps a domain slice to the models slice.
func PromptTemplateListFromDomain(list []analysis.PromptTemplate) []models.PromptTemplate {
	out := make([]models.PromptTemplate, 0, len(list))
	for i := range list {
		out = append(out, *PromptTemplateFromDomain(&list[i]))
	}
	return out
}

// PromptTemplatePageDataFromDomain maps a domain page to the models page.
func PromptTemplatePageDataFromDomain(p *analysis.PromptTemplatePageData) *models.PromptTemplatePageData {
	if p == nil {
		return nil
	}
	out := &models.PromptTemplatePageData{
		Total:      p.Total,
		Page:       p.Page,
		PageSize:   p.PageSize,
		TotalPages: p.TotalPages,
	}
	for i := range p.List {
		out.List = append(out.List, *PromptTemplateFromDomain(&p.List[i]))
	}
	return out
}
