// Package jobsync checks tracked companies for new matching jobs and
// notifies about them over Telegram.
package jobsync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/fetcher"
	"github.com/kv13b/Crontfetch/internal/models"
	"github.com/kv13b/Crontfetch/internal/telegram"
)

// maxPagesPerFetch mirrors the handler's cap, so the background sync reads
// the same amount of a career site as a manual jobs request would.
const maxPagesPerFetch = 100

// SyncAll checks every tracked company (across all users) for new matching
// jobs and notifies about them. It logs and continues past a single
// company's failure, so one broken career site doesn't stop the rest.
func SyncAll(ctx context.Context, pool *pgxpool.Pool, botToken, chatID string) {
	companies, err := listAllCompanies(ctx, pool)
	if err != nil {
		slog.Error("jobsync: listing companies", "error", err)
		return
	}

	slog.Info("jobsync: starting sync", "companies", len(companies))
	for _, c := range companies {
		if err := syncCompany(ctx, pool, botToken, chatID, c); err != nil {
			slog.Error("jobsync: syncing company", "error", err, "company_id", c.ID, "name", c.Name)
		}
	}
}

// SyncCompanyByID runs the same check-and-notify cycle as SyncAll, but for
// one company. Useful for an on-demand "check now" trigger, or to verify
// the sync logic against a single company without waiting for a full run.
func SyncCompanyByID(ctx context.Context, pool *pgxpool.Pool, botToken, chatID, companyID string) error {
	c, err := getCompanyByID(ctx, pool, companyID)
	if err != nil {
		return fmt.Errorf("looking up company: %w", err)
	}
	return syncCompany(ctx, pool, botToken, chatID, c)
}

func syncCompany(ctx context.Context, pool *pgxpool.Pool, botToken, chatID string, c models.Company) error {
	jobs, err := fetcher.FetchJobs(ctx, c.CareerURL, c.Platform, c.Board, maxPagesPerFetch)
	if err != nil {
		return fmt.Errorf("fetching jobs: %w", err)
	}
	matched := fetcher.FilterJobs(jobs, c.Roles, c.Locations)

	// A company with no rows yet is being synced for the first time: record
	// its current matches as a baseline instead of notifying about all of
	// them at once, since they aren't newly posted, just newly tracked.
	seenBefore, err := hasSeenJobs(ctx, pool, c.ID)
	if err != nil {
		return fmt.Errorf("checking sync history: %w", err)
	}

	newJobs, err := recordNewJobs(ctx, pool, c.ID, matched)
	if err != nil {
		return fmt.Errorf("recording jobs: %w", err)
	}

	if !seenBefore {
		slog.Info("jobsync: baseline recorded", "company_id", c.ID, "name", c.Name, "matched", len(matched))
		return nil
	}

	slog.Info("jobsync: synced", "company_id", c.ID, "name", c.Name, "matched", len(matched), "new", len(newJobs))
	for _, j := range newJobs {
		text := fmt.Sprintf("New job at %s: %s (%s)\n%s", c.Name, j.Title, j.Location, j.URL)
		if err := telegram.SendMessage(ctx, botToken, chatID, text); err != nil {
			// Keep notifying about the rest; the job is already recorded,
			// so a retry of the whole sync wouldn't re-send this one anyway.
			slog.Error("jobsync: sending telegram notification", "error", err, "company_id", c.ID, "job_id", j.ID)
		}
	}
	return nil
}
