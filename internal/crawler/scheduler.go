package crawler

import (
	"context"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/metrics"
)

// Scheduler provides basic cron-style scheduling logic (§39).
type Scheduler struct {
	intervals map[string]time.Duration
	logger    *metrics.Logger
}

// NewScheduler creates a new scheduler with default intervals.
func NewScheduler(logger *metrics.Logger) *Scheduler {
	return &Scheduler{
		intervals: map[string]time.Duration{
			"github":      24 * time.Hour, // incremental every 24h
			"registry":    24 * time.Hour,
			"full":        168 * time.Hour, // full crawl weekly
		},
		logger: logger,
	}
}

// ShouldRun returns true if the source should run now based on the schedule.
func (s *Scheduler) ShouldRun(source string, lastRun time.Time) bool {
	interval, ok := s.intervals[source]
	if !ok {
		interval = 24 * time.Hour
	}
	return time.Since(lastRun) >= interval
}

// ScheduleFullCrawl returns the next full crawl time (Sunday 02:00).
func (s *Scheduler) ScheduleFullCrawl() time.Time {
	now := time.Now()
	daysUntilSunday := (7 - int(now.Weekday())) % 7
	if daysUntilSunday == 0 && now.Hour() >= 2 {
		daysUntilSunday = 7
	}
	next := time.Date(now.Year(), now.Month(), now.Day()+daysUntilSunday, 2, 0, 0, 0, now.Location())
	return next
}

// RunPeriodic runs the coordinator periodically until context is cancelled.
func (s *Scheduler) RunPeriodic(ctx context.Context, coord *CrawlCoordinator) error {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := coord.Run(ctx, CrawlOptions{Source: "all", FullCrawl: false}); err != nil {
				s.logger.Error(ctx, "", "scheduler", "crawl_failed", "error", err.Error())
			}
		}
	}
}
