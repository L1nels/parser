package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/network"
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
		zap.L().Fatal("Ошибка подключения к БД", zap.Error(err))
	}
	defer dbPool.Close()

	// Создаём один «большой» корневой контекст браузера
	opts := chromedp.DefaultExecAllocatorOptions[:]
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// Внешний браузерный контекст
	browserCtx, browserCancel := chromedp.NewContext(ctx)
	defer browserCancel()

	// Запускаем бесконечный (или пока) цикл
	for {
		err := runIterationWithRetries(browserCtx, cfg, dbPool)
		if err != nil {
			zap.L().Error("Ошибка при итерации, прекращаем работу", zap.Error(err))
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	zap.L().Info("Парсер завершил работу.")
}

// runIterationWithRetries — если в итерации ошибка, пытаемся до 3 раз
func runIterationWithRetries(ctx context.Context, cfg *Config, dbPool *pgxpool.Pool) error {
	const maxRetries = 3
	var lastErr error

	for i := 1; i <= maxRetries; i++ {
		err := processIteration(ctx, cfg, dbPool)
		if err == nil {
			return nil
		}
		zap.L().Warn("Ошибка в processIteration",
			zap.Error(err),
			zap.Int("retry", i),
		)
		lastErr = err
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("превышено число ретраев (%d). последняя ошибка: %w", maxRetries, lastErr)
}

// processIteration — одна итерация (новый контекст, network.Enable, SolveCaptcha, парсинг)
func processIteration(parentCtx context.Context, cfg *Config, dbPool *pgxpool.Pool) error {
	// 1) Создаём «свежий» дочерний контекст
	iterCtx, iterCancel := chromedp.NewContext(parentCtx)
	defer iterCancel()

	// 2) Включаем network-домен (чтобы GetCookies работал в этом контексте)
	if err := chromedp.Run(iterCtx, network.Enable()); err != nil {
		return fmt.Errorf("не удалось включить network в контексте: %w", err)
	}

	// 3) Решаем капчу (в iterCtx)
	if err := SolveCaptcha(iterCtx, cfg); err != nil {
		return err
	}

	// 4) Забираем HTML
	htmlContent, err := GetHTML(iterCtx, cfg.LiveFootballURL)
	if err != nil {
		return err
	}

	// 5) Ищем динамический хост
	dynamicHost, err := GetDynamicHost(htmlContent)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://%s/events/list?lang=ru&scopeMarket=1600", dynamicHost)

	// 6) Данные из API
	data, err := FetchData(apiURL)
	if err != nil {
		return err
	}

	// 7) Сохраняем в БД
	if err := SaveDataToPostgres(dbPool, *data); err != nil {
		return err
	}

	zap.L().Debug("Итерация завершена без ошибок")
	return nil
}
