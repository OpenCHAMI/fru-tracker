package storage

import (
	"context"
	"fmt"

	"github.com/example/fru-tracker/internal/storage/ent"
)

// WithTx executes fn within a database transaction.
// If fn returns an error, the transaction is rolled back; otherwise, it is committed.
func WithTx(ctx context.Context, fn func(*ent.Tx) error) error {
	if entClient == nil {
		return fmt.Errorf("ent client not initialized")
	}
	tx, err := entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if v := recover(); v != nil {
			_ = tx.Rollback()
			panic(v)
		}
	}()
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return fmt.Errorf("%w: rollback error: %v", err, rerr)
		}
		return err
	}
	return tx.Commit()
}
