package services

import (
	"applegacy/backend/internal/core/domain"
	"applegacy/backend/internal/core/ports"
	"context"
)

type StatsService struct {
	repo   ports.StatsRepository
	crypto ports.CryptoService
}

func NewStatsService(repo ports.StatsRepository, crypto ports.CryptoService) *StatsService {
	return &StatsService{
		repo:   repo,
		crypto: crypto,
	}
}

func (s *StatsService) GetDashboardStats(ctx context.Context) (*domain.DashboardStats, error) {
	topArticles, err := s.repo.GetTopArticles(ctx, 10)
	if err != nil {
		return nil, err
	}

	topUsers, err := s.repo.GetTopUsers(ctx, 10)
	if err != nil {
		return nil, err
	}

	weekly, err := s.repo.GetViewsByPeriod(ctx, "week")
	if err != nil {
		return nil, err
	}

	monthly, err := s.repo.GetViewsByPeriod(ctx, "month")
	if err != nil {
		return nil, err
	}

	yearly, err := s.repo.GetViewsByPeriod(ctx, "year")
	if err != nil {
		return nil, err
	}

	// Decrypt user names
	for i := range topUsers {
		if decryptedFirst, err := s.crypto.Decrypt(topUsers[i].FirstName); err == nil {
			topUsers[i].FirstName = decryptedFirst
		}
		if decryptedLast, err := s.crypto.Decrypt(topUsers[i].LastName); err == nil {
			topUsers[i].LastName = decryptedLast
		}
	}

	return &domain.DashboardStats{
		TopArticles:  topArticles,
		TopUsers:     topUsers,
		WeeklyStats:  weekly,
		MonthlyStats: monthly,
		YearlyStats:  yearly,
	}, nil
}
