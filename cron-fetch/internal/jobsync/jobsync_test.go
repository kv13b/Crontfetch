package jobsync

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestSyncCompanyByID_NotifiesOnlyNewJobs is a manual verification tool, not
// part of routine CI: it needs a real database with an already-tracked
// company (so it has a baseline to compare against), a real Telegram bot,
// and it actually sends a message. Point it at one via env vars:
//
//	DATABASE_URL, TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID, JOBSYNC_TEST_COMPANY_ID
//
// It deletes one already-recorded job for that company (simulating "this
// one looks new again"), re-syncs, and checks that exactly that job was
// re-recorded and notified about — proving new jobs get a Telegram message
// while everything else stays quiet.
func TestSyncCompanyByID_NotifiesOnlyNewJobs(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	companyID := os.Getenv("JOBSYNC_TEST_COMPANY_ID")
	if dbURL == "" || token == "" || chatID == "" || companyID == "" {
		t.Skip("DATABASE_URL/TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID/JOBSYNC_TEST_COMPANY_ID not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	var externalID string
	err = pool.QueryRow(ctx, "SELECT external_id FROM jobs WHERE company_id = $1 LIMIT 1", companyID).Scan(&externalID)
	if err != nil {
		t.Fatalf("finding an existing job to remove: %v (does this company have a recorded baseline yet?)", err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM jobs WHERE company_id = $1 AND external_id = $2", companyID, externalID); err != nil {
		t.Fatalf("removing job %s: %v", externalID, err)
	}
	t.Logf("removed job %s to simulate it appearing as new", externalID)

	if err := SyncCompanyByID(ctx, pool, token, chatID, companyID); err != nil {
		t.Fatalf("SyncCompanyByID() error = %v", err)
	}

	var restored bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM jobs WHERE company_id = $1 AND external_id = $2)", companyID, externalID).Scan(&restored)
	if err != nil {
		t.Fatalf("checking job was re-recorded: %v", err)
	}
	if !restored {
		t.Fatalf("job %s was not re-recorded after syncing — it should have been treated as new", externalID)
	}
	t.Logf("job %s was re-recorded; check Telegram for its notification", externalID)
}
