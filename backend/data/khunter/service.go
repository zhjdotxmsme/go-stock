package khunter

import (
	"context"
	"sort"

	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/models"
)

// Service khunter 对外门面，handler 只调它
type Service struct {
	repo *Repo
}

func NewService() *Service {
	_ = EnsureMigrate()
	return &Service{repo: NewRepo()}
}

func (s *Service) RunPipeline(tradeDate string) (*PipelineResult, error) {
	return RunPipeline(context.Background(), tradeDate)
}

func (s *Service) GetScores(date string) ([]models.KhunterScore, error) {
	return s.repo.GetScoresByDate(date)
}

func (s *Service) GetSignals(date string) ([]models.KhunterSignal, error) {
	return s.repo.GetSignalsByDate(date)
}

func (s *Service) GetHunting(status string) ([]models.KhunterHunting, error) {
	return s.repo.GetHuntingList(status)
}

func (s *Service) GetRiskLevel() (*models.KhunterRiskLevel, error) {
	return s.repo.GetLatestRiskLevel()
}

// KellySuggestion 单策略半凯利建议仓位（spec D6 / §9 Tab4）。
// json tag 显式 camelCase——wails 序列化不做名称转换。
type KellySuggestion struct {
	Strategy string  `json:"strategy"`
	WinRate  float64 `json:"winRate"`
	PLRatio  float64 `json:"plRatio"`
	Fraction float64 `json:"fraction"`
}

// kellyConfigPath 相对进程工作目录定位配置文件。
// 已知限制：进程需从仓库根目录（或 config/ 所在目录的父目录）启动，
// 否则 LoadKellyConfig 返回 nil，本方法返回空列表。
const kellyConfigPath = "config/khunter_kelly.json"

// GetKellySuggestions 读取各策略 [胜率, 盈亏比] 配置并计算半凯利仓位；
// 配置缺失/解析失败返回空切片（前端显示"未配置凯利参数"）。
// TODO(follow-up): 胜率与盈亏比应回测链路自动回填，当前为静态配置。
func (s *Service) GetKellySuggestions() []KellySuggestion {
	cfg := risk.LoadKellyConfig(kellyConfigPath)
	out := make([]KellySuggestion, 0, len(cfg))
	for name, wb := range cfg {
		out = append(out, KellySuggestion{
			Strategy: name,
			WinRate:  wb[0],
			PLRatio:  wb[1],
			Fraction: risk.KellyFraction(wb[0], wb[1]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Strategy < out[j].Strategy })
	return out
}
