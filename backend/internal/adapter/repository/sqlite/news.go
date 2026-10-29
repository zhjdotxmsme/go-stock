package sqlite

import (
	"context"

	"go-stock/backend/db"
	"go-stock/backend/internal/domain/market"
	"go-stock/backend/models"
)

// TelegraphRepository implements repository.TelegraphRepository.
// 查询与标签补全逻辑复刻原 data.MarketNewsApi.GetTelegraphList。
type TelegraphRepository struct{}

// NewTelegraphRepository creates a new TelegraphRepository.
func NewTelegraphRepository() *TelegraphRepository {
	return &TelegraphRepository{}
}

func (r *TelegraphRepository) GetTelegraphList(ctx context.Context, source string, limit int) ([]market.Telegraph, error) {
	news := make([]market.Telegraph, 0)
	q := db.Dao.Model(&market.Telegraph{}).Preload("TelegraphTags").Order("data_time desc,time desc").Limit(limit)
	if source != "" {
		q = q.Where("source=?", source)
	}
	if err := q.Find(&news).Error; err != nil {
		return nil, err
	}

	// SubjectTags 批量补全：telegraph_tags -> tags 名称。
	// 原实现逐条查询（50 条 = 50 次查询），改为两条批量查询避免 N+1。
	tagIDs := make([]uint, 0)
	for i := range news {
		for _, t := range news[i].TelegraphTags {
			tagIDs = append(tagIDs, t.TagId)
		}
	}
	if len(tagIDs) > 0 {
		tags := make([]market.Tags, 0)
		if err := db.Dao.Model(&market.Tags{}).Where("id in ?", tagIDs).Find(&tags).Error; err != nil {
			return nil, err
		}
		names := make(map[uint]string, len(tags))
		for _, t := range tags {
			names[t.ID] = t.Name
		}
		for i := range news {
			set := make(map[uint]struct{}, len(news[i].TelegraphTags))
			for _, t := range news[i].TelegraphTags {
				set[t.TagId] = struct{}{}
			}
			subjectTags := make([]string, 0, len(set))
			for id := range set {
				if name, ok := names[id]; ok {
					subjectTags = append(subjectTags, name)
				}
			}
			news[i].SubjectTags = subjectTags
		}
	}
	return news, nil
}

// ---------------------------------------------------------------------------
// models <-> domain 显式映射（不反射）
// ---------------------------------------------------------------------------

// TelegraphFromDomain maps a domain model to the data-layer GORM model.
func TelegraphFromDomain(t *market.Telegraph) *models.Telegraph {
	if t == nil {
		return nil
	}
	out := &models.Telegraph{
		Model:           t.Model,
		Time:            t.Time,
		DataTime:        t.DataTime,
		Title:           t.Title,
		Content:         t.Content,
		SubjectTags:     t.SubjectTags,
		StocksTags:      t.StocksTags,
		IsRed:           t.IsRed,
		Url:             t.Url,
		Source:          t.Source,
		SentimentResult: t.SentimentResult,
	}
	for _, tag := range t.TelegraphTags {
		out.TelegraphTags = append(out.TelegraphTags, models.TelegraphTags{
			Model:       tag.Model,
			TagId:       tag.TagId,
			TelegraphId: tag.TelegraphId,
		})
	}
	return out
}

// TelegraphPtrListFromDomain maps a domain slice to a models pointer slice
// (handler 对外契约类型为 *[]*models.Telegraph)。
func TelegraphPtrListFromDomain(list []market.Telegraph) []*models.Telegraph {
	out := make([]*models.Telegraph, 0, len(list))
	for i := range list {
		out = append(out, TelegraphFromDomain(&list[i]))
	}
	return out
}
