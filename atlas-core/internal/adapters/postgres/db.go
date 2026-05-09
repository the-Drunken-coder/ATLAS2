package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anomalyco/atlas-core/internal/config"
	"github.com/anomalyco/atlas-core/internal/logging"
)

func NewPool(ctx context.Context, cfg *config.Config, log *logging.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.PostgresDSN())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	poolCfg.MaxConns = cfg.PostgresMaxConns
	poolCfg.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	log.InfoContext(ctx, "postgres", "connected to PostgreSQL", logging.Any("max_conns", cfg.PostgresMaxConns))
	return pool, nil
}

func Begin(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	return pool.BeginTx(ctx, pgx.TxOptions{})
}

func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := Begin(ctx, pool)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	fnErr := fn(tx)
	if fnErr != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return errors.Join(fnErr, fmt.Errorf("rollback transaction: %w", rbErr))
		}
		return fnErr
	}
	if err := tx.Commit(ctx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return errors.Join(fmt.Errorf("commit transaction: %w", err), fmt.Errorf("rollback after failed commit: %w", rbErr))
		}
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
