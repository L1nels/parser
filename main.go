package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	cfg, err := LoadConfig()
	if err != nil {
		zap.L().Fatal("Не удалось загрузить конфигурацию", zap.Error(err))
	}

	dbPool, err := ConnectToPostgres(cfg)
	if err != nil {
		zap.L().Fatal("Ошибка подключения к PostgreSQL", zap.Error(err))
	}
	defer dbPool.Close()

	opts := chromedp.DefaultExecAllocatorOptions[:]
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	browserCtx, browserCancel := chromedp.NewContext(ctx)
	defer browserCancel()

	for {
		if err := processIteration(browserCtx, cfg, dbPool); err != nil {
			zap.L().Error("Ошибка в процессе итерации", zap.Error(err))
		}

		// Добавляем задержку ~200 мс, чтобы не банили
		time.Sleep(200 * time.Millisecond)
	}
}

func processIteration(ctx context.Context, cfg *Config, dbPool *pgxpool.Pool) error {
	// 1. Решаем капчу (если есть)
	err := SolveCaptcha(ctx, cfg)
	if err != nil {
		// Если капча не найдена или ошибка, логируем, но не прерываем полностью парсинг
		zap.L().Warn("Капча не решена / ошибка капчи", zap.Error(err))
	}

	// 2. Забираем HTML
	htmlContent, err := GetHTML(ctx, cfg.LiveFootballURL)
	if err != nil {
		return fmt.Errorf("ошибка GetHTML: %w", err)
	}

	// 3. Достаём динамический хост
	dynamicHost, err := GetDynamicHost(htmlContent)
	if err != nil {
		return fmt.Errorf("ошибка GetDynamicHost: %w", err)
	}

	apiURL := fmt.Sprintf("https://%s/events/list?lang=ru&scopeMarket=1600", dynamicHost)

	// 4. Запрашиваем данные
	data, err := FetchData(apiURL)
	if err != nil {
		return fmt.Errorf("ошибка FetchData: %w", err)
	}

	// 5. Сохраняем в БД
	if err := SaveDataToPostgres(dbPool, *data); err != nil {
		return fmt.Errorf("ошибка SaveDataToPostgres: %w", err)
	}

	zap.L().Debug("Итерация успешно завершена")
	return nil
}
