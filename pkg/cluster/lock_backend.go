package cluster

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SQLiteLockBackend implements LockBackend using SQLite
type SQLiteLockBackend struct {
	db *sql.DB
}

// NewSQLiteLockBackend creates a backend using the given sqlite connection
func NewSQLiteLockBackend(db *sql.DB) *SQLiteLockBackend {
	return &SQLiteLockBackend{db: db}
}

// Migrate creates the distributed_locks table if not exists
func (b *SQLiteLockBackend) Migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS distributed_locks (
	key TEXT PRIMARY KEY,
	holder TEXT NOT NULL,
	expires_at DATETIME NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`
	_, err := b.db.Exec(schema)
	return err
}

// TryAcquire attempts to insert or update the lock row atomically
func (b *SQLiteLockBackend) TryAcquire(ctx context.Context, key, holder string, ttl time.Duration) (bool, error) {
	expires := time.Now().Add(ttl).UTC()

	// Use a transaction for atomicity
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var existingHolder string
	var existingExpires time.Time
	err = tx.QueryRowContext(ctx, "SELECT holder, expires_at FROM distributed_locks WHERE key = ?", key).Scan(&existingHolder, &existingExpires)
	if err != nil {
		if err == sql.ErrNoRows {
			// No existing lock — insert
			_, err = tx.ExecContext(ctx, "INSERT INTO distributed_locks(key, holder, expires_at) VALUES (?, ?, ?)", key, holder, expires)
			if err != nil {
				return false, err
			}
			if err := tx.Commit(); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, err
	}

	// Existing lock: check if expired (use Before instead of After for clarity)
	if time.Now().UTC().After(existingExpires) {
		// Expired — take over
		_, err = tx.ExecContext(ctx, "UPDATE distributed_locks SET holder = ?, expires_at = ? WHERE key = ?", holder, expires, key)
		if err != nil {
			return false, err
		}
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return true, nil
	}

	// Lock held and not expired
	return false, nil
}

// Release deletes the lock row if held by the given holder
func (b *SQLiteLockBackend) Release(ctx context.Context, key, holder string) error {
	res, err := b.db.ExecContext(ctx, "DELETE FROM distributed_locks WHERE key = ? AND holder = ?", key, holder)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("lock not held by %s", holder)
	}
	return nil
}

// Refresh updates the expiry time
func (b *SQLiteLockBackend) Refresh(ctx context.Context, key, holder string, ttl time.Duration) error {
	expires := time.Now().Add(ttl).UTC()
	res, err := b.db.ExecContext(ctx, "UPDATE distributed_locks SET expires_at = ? WHERE key = ? AND holder = ?", expires, key, holder)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("lock not held by %s", holder)
	}
	return nil
}
