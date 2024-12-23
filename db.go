package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
)

func ConnectToPostgres(cfg *Config) (*pgxpool.Pool, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		cfg.PostgresUser, cfg.PostgresPassword,
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB,
	)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга подключения: %w", err)
	}

	connPool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %w", err)
	}

	if err := initSchema(connPool); err != nil {
		return nil, fmt.Errorf("ошибка initSchema: %w", err)
	}

	zap.L().Info("Подключение к PostgreSQL успешно установлено!")
	return connPool, nil
}

func initSchema(pool *pgxpool.Pool) error {
	schema := `
CREATE TABLE IF NOT EXISTS events (
    id BIGINT PRIMARY KEY,
    parent_id BIGINT,
    name TEXT,
    sport_id BIGINT,
    start_time BIGINT,
    place TEXT,
    priority INT
);
`
	_, err := pool.Exec(context.Background(), schema)
	return err
}

func SaveDataToPostgres(connPool *pgxpool.Pool, data ApiResponse) error {
	ctx := context.Background()

	for _, e := range data.Events {
		_, err := connPool.Exec(ctx,
			`INSERT INTO events (id, parent_id, name, sport_id, start_time, place, priority)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (id) DO NOTHING`,
			e.ID, e.ParentID, e.Name, e.SportID, e.StartTime, e.Place, e.Priority)
		if err != nil {
			zap.L().Error("Ошибка вставки события", zap.Error(err),
				zap.Int64("event_id", e.ID))
			continue
		}
		zap.L().Debug("Событие сохранено (ON CONFLICT DO NOTHING)",
			zap.Int64("event_id", e.ID))
	}
	return nil
}
