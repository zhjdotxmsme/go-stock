package khunter

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"go-stock/backend/data"
	"go-stock/backend/data/khunter/risk"
	"go-stock/backend/models"
	"go-stock/backend/stockcode"
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

// KhunterMeta 狩猎场数据日期提示（前端默认选中有数据的交易日）
type KhunterMeta struct {
	LatestScoreDate  string `json:"latestScoreDate"`
	LatestSignalDate string `json:"latestSignalDate"`
}

func (s *Service) GetMeta() KhunterMeta {
	scoreDate, _ := s.repo.GetLatestScoreDate()
	sigDate, _ := s.repo.GetLatestSignalDate()
	return KhunterMeta{LatestScoreDate: scoreDate, LatestSignalDate: sigDate}
}

func (s *Service) GetScores(date string) ([]models.KhunterScore, error) {
	rows, err := s.repo.GetScoresByDate(date)
	if err != nil {
		return nil, err
	}
	enrichKhunterScoreNames(rows)
	return rows, nil
}

func (s *Service) GetSignals(date string) ([]models.KhunterSignal, error) {
	rows, err := s.repo.GetSignalsByDate(date)
	if err != nil {
		return nil, err
	}
	enrichKhunterSignalNames(rows)
	return rows, nil
}

func (s *Service) GetHunting(status string) ([]models.KhunterHunting, error) {
	rows, err := s.repo.GetHuntingList(status)
	if err != nil {
		return nil, err
	}
	enrichKhunterHuntingNames(rows)
	return rows, nil
}

func enrichKhunterScoreNames(rows []models.KhunterScore) {
	missing := make([]string, 0)
	for _, r := range rows {
		if r.Name == "" && r.Code != "" {
			missing = append(missing, r.Code)
		}
	}
	if len(missing) == 0 {
		return
	}
	names := data.ResolveStockNames(missing)
	for i := range rows {
		if rows[i].Name != "" {
			continue
		}
		if n := names[stockcode.Normalize(rows[i].Code)]; n != "" {
			rows[i].Name = n
		}
	}
}

func enrichKhunterSignalNames(rows []models.KhunterSignal) {
	missing := make([]string, 0)
	for _, r := range rows {
		if r.Name == "" && r.Code != "" {
			missing = append(missing, r.Code)
		}
	}
	if len(missing) == 0 {
		return
	}
	names := data.ResolveStockNames(missing)
	for i := range rows {
		if rows[i].Name != "" {
			continue
		}
		if n := names[stockcode.Normalize(rows[i].Code)]; n != "" {
			rows[i].Name = n
		}
	}
}

func enrichKhunterHuntingNames(rows []models.KhunterHunting) {
	missing := make([]string, 0)
	for _, r := range rows {
		if r.Name == "" && r.Code != "" {
			missing = append(missing, r.Code)
		}
	}
	if len(missing) == 0 {
		return
	}
	names := data.ResolveStockNames(missing)
	for i := range rows {
		if rows[i].Name != "" {
			continue
		}
		if n := names[stockcode.Normalize(rows[i].Code)]; n != "" {
			rows[i].Name = n
		}
	}
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
const kellyConfigRel = "config/khunter_kelly.json"

func resolveKellyConfigPath() string {
	if _, err := os.Stat(kellyConfigRel); err == nil {
		return kellyConfigRel
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), kellyConfigRel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return kellyConfigRel
}

// GetKellySuggestions 读取各策略 [胜率, 盈亏比] 配置并计算半凯利仓位；
// 配置缺失/解析失败返回空切片（前端显示"未配置凯利参数"）。
// TODO(follow-up): 胜率与盈亏比应回测链路自动回填，当前为静态配置。
func (s *Service) GetKellySuggestions() []KellySuggestion {
	cfg := risk.LoadKellyConfig(resolveKellyConfigPath())
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
