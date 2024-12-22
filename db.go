package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
)

// ConnectToPostgres — подключение к PostgreSQL, создаём connection pool + initSchema
func ConnectToPostgres(cfg *Config) (*pgxpool.Pool, error) {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга конфигурации подключения: %w", err)
	}

	connPool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	err = initSchema(connPool)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации схемы: %w", err)
	}

	zap.L().Info("Подключение к PostgreSQL успешно установлено!")
	return connPool, nil
}

// initSchema — создаём (если нет) таблицу events
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

// SaveDataToPostgres — вставляем события в таблицу
func SaveDataToPostgres(connPool *pgxpool.Pool, data ApiResponse) error {
	ctx := context.Background()

	for _, e := range data.Events {
		_, err := connPool.Exec(ctx,
			`INSERT INTO events (id, parent_id, name, sport_id, start_time, place, priority)
             VALUES ($1, $2, $3, $4, $5, $6, $7)
             ON CONFLICT (id) DO NOTHING`,
			e.ID, e.ParentID, e.Name, e.SportID, e.StartTime, e.Place, e.Priority)
		if err != nil {
			zap.L().Error("Ошибка вставки события в БД",
				zap.Error(err),
				zap.Int64("event_id", e.ID),
			)
			// Если хотим сразу прерывать сохранение всех событий при первой ошибке,
			// можно вернуть err. Но если хотим пропускать только сбойные, оставляем continue.
			continue
		}
		zap.L().Debug("Событие сохранено (или пропущено ON CONFLICT)",
			zap.Int64("event_id", e.ID),
		)
	}

	return nil
}
