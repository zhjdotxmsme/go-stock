package khunter

import (
	"context"

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
