package data

import (
	"context"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DailyPickRepository owns all persistence for the daily-pick feature
// (S4 step 2). Service/review/engine depend on this type instead of the
// global db.Dao, so the queries are injectable and testable; the query
// bodies moved here verbatim from their original call sites.
type DailyPickRepository struct {
	db *gorm.DB
}

// NewDailyPickRepository captures the global handle at construction time.
func NewDailyPickRepository() *DailyPickRepository {
	return &DailyPickRepository{db: db.Dao}
}

// ---- Queries (service) ----

// QueryDailyPicks applies the filters/pagination of query and returns
// (total, page rows).
func (r *DailyPickRepository) QueryDailyPicks(ctx context.Context, query models.DailyPickQuery) (int64, []models.DailyPick, error) {
	var total int64
	var picks []models.DailyPick

	tx := r.db.WithContext(ctx).Model(&models.DailyPick{})
	if query.TradeDate != "" {
		tx = tx.Where("trade_date = ?", query.TradeDate)
	}
	if query.StartDate != "" {
		tx = tx.Where("trade_date >= ?", query.StartDate)
	}
	if query.EndDate != "" {
		tx = tx.Where("trade_date <= ?", query.EndDate)
	}
	if query.Reviewed != nil {
		tx = tx.Where("reviewed = ?", *query.Reviewed)
	}

	if err := tx.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := tx.Order("trade_date DESC, score DESC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&picks).Error
	return total, picks, err
}

// TodayTopPicks returns today's top picks by score.
func (r *DailyPickRepository) TodayTopPicks(ctx context.Context, today string, topN int) ([]models.DailyPick, error) {
	var picks []models.DailyPick
	err := r.db.WithContext(ctx).Where("trade_date = ?", today).
		Order("score DESC").
		Limit(topN).
		Find(&picks).Error
	return picks, err
}

// LatestPick returns the pick with the most recent trade date.
func (r *DailyPickRepository) LatestPick(ctx context.Context) (*models.DailyPick, error) {
	var latest models.DailyPick
	if err := r.db.WithContext(ctx).Order("trade_date DESC").First(&latest).Error; err != nil {
		return nil, err
	}
	return &latest, nil
}

// PicksByDateTop returns up to topN picks of a date ordered by score.
func (r *DailyPickRepository) PicksByDateTop(ctx context.Context, tradeDate string, topN int) []models.DailyPick {
	var picks []models.DailyPick
	r.db.WithContext(ctx).Where("trade_date = ?", tradeDate).
		Order("score DESC").
		Limit(topN).
		Find(&picks)
	return picks
}

// DeletePick removes a pick by id.
func (r *DailyPickRepository) DeletePick(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.DailyPick{}, id).Error
}

// UpdateRemarks updates the remarks field of a pick.
func (r *DailyPickRepository) UpdateRemarks(ctx context.Context, id uint, remarks string) error {
	return r.db.WithContext(ctx).Model(&models.DailyPick{}).Where("id = ?", id).Update("remarks", remarks).Error
}

// CountPicks returns (total, reviewed-count).
func (r *DailyPickRepository) CountPicks(ctx context.Context) (int64, int64) {
	var totalPicks64, reviewedPicks64 int64
	r.db.WithContext(ctx).Model(&models.DailyPick{}).Count(&totalPicks64)
	r.db.WithContext(ctx).Model(&models.DailyPick{}).Where("reviewed = ?", true).Count(&reviewedPicks64)
	return totalPicks64, reviewedPicks64
}

// FindReviewed returns all reviewed picks.
func (r *DailyPickRepository) FindReviewed(ctx context.Context) []models.DailyPick {
	var picks []models.DailyPick
	r.db.WithContext(ctx).Where("reviewed = ?", true).Find(&picks)
	return picks
}

// LatestUnreviewed returns the unreviewed pick with the most recent date.
func (r *DailyPickRepository) LatestUnreviewed(ctx context.Context) (*models.DailyPick, error) {
	var latest models.DailyPick
	if err := r.db.WithContext(ctx).Where("reviewed = ?", false).
		Order("trade_date DESC").
		First(&latest).Error; err != nil {
		return nil, err
	}
	return &latest, nil
}

// UnreviewedByDate returns unreviewed picks of a date ordered by score.
func (r *DailyPickRepository) UnreviewedByDate(ctx context.Context, tradeDate string) []models.DailyPick {
	var picks []models.DailyPick
	r.db.WithContext(ctx).Where("trade_date = ? AND reviewed = ?", tradeDate, false).
		Order("score DESC").
		Find(&picks)
	return picks
}

// DateRange returns the earliest and latest trade dates with picks.
func (r *DailyPickRepository) DateRange(ctx context.Context) (string, string) {
	var first, last models.DailyPick
	start, end := "", ""
	if err := r.db.WithContext(ctx).Order("trade_date ASC").First(&first).Error; err == nil {
		start = first.TradeDate
	}
	if err := r.db.WithContext(ctx).Order("trade_date DESC").First(&last).Error; err == nil {
		end = last.TradeDate
	}
	return start, end
}

// ---- Queries (review) ----

// FindUnreviewedByDate returns unreviewed picks of a date (insertion order).
func (r *DailyPickRepository) FindUnreviewedByDate(ctx context.Context, tradeDate string) []models.DailyPick {
	var picks []models.DailyPick
	r.db.WithContext(ctx).
		Where("trade_date = ? AND reviewed = ?", tradeDate, false).
		Find(&picks)
	return picks
}

// EarliestUnreviewed returns the unreviewed pick with the earliest date.
func (r *DailyPickRepository) EarliestUnreviewed(ctx context.Context) (*models.DailyPick, error) {
	var pick models.DailyPick
	if err := r.db.WithContext(ctx).
		Where("reviewed = ?", false).
		Order("trade_date ASC").
		First(&pick).Error; err != nil {
		return nil, err
	}
	return &pick, nil
}

// SavePick persists the full pick row.
func (r *DailyPickRepository) SavePick(ctx context.Context, pick *models.DailyPick) error {
	return r.db.WithContext(ctx).Save(pick).Error
}

// ReviewedPicks returns reviewed picks, optionally filtered by date.
func (r *DailyPickRepository) ReviewedPicks(ctx context.Context, tradeDate string) []models.DailyPick {
	var picks []models.DailyPick
	tx := r.db.WithContext(ctx).Where("reviewed = ?", true)
	if tradeDate != "" {
		tx = tx.Where("trade_date = ?", tradeDate)
	}
	tx.Find(&picks)
	return picks
}

// ---- Writes (engine) ----

// CreatePick persists a newly generated pick.
func (r *DailyPickRepository) CreatePick(ctx context.Context, pick *models.DailyPick) error {
	return r.db.WithContext(ctx).Create(pick).Error
}

// UpsertPick persists a pick, replacing an existing row for the same
// (stock_code, trade_date) — re-running the daily pick on the same day must
// update rather than silently drop every pick on the unique constraint.
func (r *DailyPickRepository) UpsertPick(ctx context.Context, pick *models.DailyPick) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		UpdateAll: true,
	}).Create(pick).Error
}

// IndustryConceptRow is a projection of AllStockInfo for the industry/concept map.
type IndustryConceptRow struct {
	SECUCODE string
	INDUSTRY string
	CONCEPT  string
}

// QueryAShareCandidates returns the A-share candidate universe for scoring.
func (r *DailyPickRepository) QueryAShareCandidates(ctx context.Context) ([]models.AllStockInfo, error) {
	var infos []models.AllStockInfo
	err := r.db.WithContext(ctx).
		Where("(secucode LIKE ? OR secucode LIKE ?)", "%.SH", "%.SZ").
		Where("secucode NOT LIKE ?", "688%"). // exclude 科创板
		Where("sec_uri_tynameabbr NOT LIKE ?", "%ST%").
		Where("sec_uri_tynameabbr NOT LIKE ?", "%退%").
		Find(&infos).Error
	return infos, err
}

// LoadIndustryConcept loads the secucode→industry/concept projection.
func (r *DailyPickRepository) LoadIndustryConcept(ctx context.Context) ([]IndustryConceptRow, error) {
	var infos []IndustryConceptRow
	err := r.db.WithContext(ctx).
		Model(&models.AllStockInfo{}).
		Where("secucode IS NOT NULL AND industry IS NOT NULL AND industry != ''").
		Select("secucode, industry, concept").
		Find(&infos).Error
	return infos, err
}

// EnsureAutoMigrate creates the DailyPick table when the DB is available.
func (r *DailyPickRepository) EnsureAutoMigrate() {
	if r.db == nil {
		return
	}
	ensureDailyPickAutoMigrate()
}
