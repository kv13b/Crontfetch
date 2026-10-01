package jobsync

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kv13b/Crontfetch/internal/fetcher"
	"github.com/kv13b/Crontfetch/internal/models"
)

// listAllCompanies returns every tracked company across all users, since
// syncing is a background job rather than a per-user request.
func listAllCompanies(ctx context.Context, pool *pgxpool.Pool) ([]models.Company, error) {
	const query = `
		SELECT id, name, career_url, platform, board, min_experience_years, max_experience_years, roles, locations
		FROM companies
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	companies := make([]models.Company, 0)
	for rows.Next() {
		c, err := scanSyncCompany(rows)
		if err != nil {
			return nil, err
		}
		companies = append(companies, c)
	}
	return companies, rows.Err()
}

// getCompanyByID reads one company by id, regardless of owner — jobsync is
// a background job, not a per-user request.
func getCompanyByID(ctx context.Context, pool *pgxpool.Pool, id string) (models.Company, error) {
	const query = `
		SELECT id, name, career_url, platform, board, min_experience_years, max_experience_years, roles, locations
		FROM companies
		WHERE id = $1
	`

	return scanSyncCompany(pool.QueryRow(ctx, query, id))
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSyncCompany(row rowScanner) (models.Company, error) {
	var c models.Company
	err := row.Scan(&c.ID, &c.Name, &c.CareerURL, &c.Platform, &c.Board,
		&c.MinExperienceYears, &c.MaxExperienceYears, &c.Roles, &c.Locations)
	return c, err
}

// hasSeenJobs reports whether any job has ever been recorded for a company,
// which distinguishes its first sync (record a baseline) from later ones
// (notify about anything new).
func hasSeenJobs(ctx context.Context, pool *pgxpool.Pool, companyID string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM jobs WHERE company_id = $1)`

	var exists bool
	err := pool.QueryRow(ctx, query, companyID).Scan(&exists)
	return exists, err
}

// recordNewJobs saves each job not already recorded for the company and
// returns just those: the ones genuinely new since the last sync. The
// company_id+external_id uniqueness constraint is what makes "not already
// recorded" safe to rely on even if two syncs somehow overlapped.
func recordNewJobs(ctx context.Context, pool *pgxpool.Pool, companyID string, jobs []fetcher.Job) ([]fetcher.Job, error) {
	const query = `
		INSERT INTO jobs (company_id, external_id, title, location, url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (company_id, external_id) DO NOTHING
	`

	var newJobs []fetcher.Job
	for _, j := range jobs {
		tag, err := pool.Exec(ctx, query, companyID, j.ID, j.Title, j.Location, j.URL)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() > 0 {
			newJobs = append(newJobs, j)
		}
	}
	return newJobs, nil
}
