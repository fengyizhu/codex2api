package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestGetAccountRecentTokenRatesByIDs(t *testing.T) {
	db, err := New("sqlite", filepath.Join(t.TempDir(), "recent-token-rate.db"))
	if err != nil {
		t.Fatalf("New(sqlite): %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	insert := func(accountID, status, duration, firstToken, output, completion int, retry bool, internal string, createdAt time.Time) {
		t.Helper()
		if _, err := db.conn.ExecContext(ctx, `INSERT INTO usage_logs
			(account_id, status_code, duration_ms, first_token_ms, output_tokens, completion_tokens,
			 is_retry_attempt, internal_reason, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			accountID, status, duration, firstToken, output, completion, retry, internal, sqliteTimeParam(createdAt)); err != nil {
			t.Fatalf("insert usage log: %v", err)
		}
	}

	// The invalid timing row is newer but must not consume one of the 20 samples.
	insert(1, 200, 0, 0, 9_999, 0, false, "", now.Add(time.Second))
	for i := 0; i < 25; i++ {
		output := 100
		completion := 0
		if i == 5 {
			// Exercise the completion_tokens fallback when output_tokens is zero.
			output = 0
			completion = 100
		}
		insert(1, 200, 1_000, 200, output, completion, false, "", now.Add(-time.Duration(i)*time.Minute))
	}
	// These rows are newer and have large token counts, but are excluded from
	// the end-user successful sample set.
	insert(1, 500, 10, 0, 99_999, 0, false, "", now.Add(2*time.Second))
	insert(1, 200, 10, 0, 99_999, 0, true, "", now.Add(3*time.Second))
	insert(1, 200, 10, 0, 99_999, 0, false, "maintenance", now.Add(4*time.Second))

	// A sub-20ms generation window falls back to the full duration.
	insert(2, 200, 100, 95, 100, 0, false, "", now)

	rates, err := db.GetAccountRecentTokenRatesByIDs(ctx, []int64{1, 2})
	if err != nil {
		t.Fatalf("GetAccountRecentTokenRatesByIDs: %v", err)
	}

	account1 := rates[1]
	if account1 == nil {
		t.Fatal("account 1 rate missing")
	}
	if account1.Samples != 20 || account1.OutputTokens != 2_000 || account1.GenerationMS != 16_000 {
		t.Fatalf("account 1 aggregate = %+v, want samples=20 tokens=2000 generation_ms=16000", account1)
	}
	if account1.TokensPerSecond != 125 {
		t.Fatalf("account 1 rate = %v, want 125 tok/s", account1.TokensPerSecond)
	}

	account2 := rates[2]
	if account2 == nil || account2.Samples != 1 || account2.GenerationMS != 100 || account2.TokensPerSecond != 1_000 {
		t.Fatalf("account 2 aggregate = %+v, want one fallback sample at 1000 tok/s", account2)
	}
}
