package main

import (
	"context"

	"github.com/chromedp/chromedp"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
)

// Глобальная переменная для хранения (можно и не использовать, если предпочитаете zap.L())
var logger *zap.SugaredLogger

func init() {
	// Можно использовать zap.NewProduction() для продакшена
	// или zap.NewDevelopment() для дебага (детальнее логи).
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	// Запоминаем (или можно заменить глобальный логгер)
	logger = l.Sugar()

	// Если хотите заменить глобальный логгер на zap, чтобы вызывать zap.L() везде:
	zap.ReplaceGlobals(l)
}

func main() {
	// Пример, как выводить в лог:
	logger.Info("Стартуем программу с zap!")

	cfg, err := LoadConfig()
	if err != nil {
		logger.Fatalw("Не удалось загрузить конфигурацию", "error", err)
	}

	dbPool, err := ConnectToPostgres(cfg)
	if err != nil {
		logger.Fatalw("Ошибка подключения к PostgreSQL", "error", err)
	}
	defer dbPool.Close()

	// Создаём контекст для chromedp
	opts := chromedp.DefaultExecAllocatorOptions[:]
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	browserCtx, browserCancel := chromedp.NewContext(ctx)
	defer browserCancel()

	// Бесконечный цикл (без задержек или с минимальной паузой)
	for {
		err := processIteration(browserCtx, cfg, dbPool)
		if err != nil {
			// Используем zap.L() или logger
			zap.L().Error("Ошибка в процессе итерации", zap.Error(err))
		}
		// Можно поставить минимальную задержку, например, 100 мс
		// time.Sleep(100 * time.Millisecond)
	}
}

func processIteration(ctx context.Context, cfg *Config, dbPool *pgxpool.Pool) error {
	// Пробуем решить капчу
	err := SolveCaptcha(ctx, cfg)
	if err != nil {
		zap.L().Warn("Возможно, капчи нет или ошибка при решении капчи", zap.Error(err))
	}

	// Получаем HTML
	htmlContent, err := GetHTML(ctx, cfg.LiveFootballURL)
	if err != nil {
		return err
	}

	// Извлекаем динамический хост
	dynamicHost, err := GetDynamicHost(htmlContent)
	if err != nil {
		return err
	}

	zap.L().Debug("Динамический хост извлечён", zap.String("host", dynamicHost))
	apiURL := "https://" + dynamicHost + "/events/list?lang=ru&scopeMarket=1600"

	// Запрашиваем данные по API
	data, err := FetchData(apiURL)
	if err != nil {
		return err
	}

	// Сохраняем в БД
	err = SaveDataToPostgres(dbPool, *data)
	if err != nil {
		return err
	}

	zap.L().Debug("Итерация завершена")
	return nil
}
