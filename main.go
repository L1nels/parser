package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
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

	// Попробуем несколько раз подключиться к БД (например, 5 попыток с 2с перерывами)
	var dbPool *pgxpool.Pool
	const maxDBRetries = 5
	for i := 1; i <= maxDBRetries; i++ {
		dbPool, err = ConnectToPostgres(cfg)
		if err == nil {
			break
		}
		zap.L().Error("Не удалось подключиться к БД, пробуем снова...", zap.Error(err), zap.Int("try", i))
		time.Sleep(2 * time.Second)
	}
	if dbPool == nil {
		zap.L().Fatal("Не смогли подключиться к БД после 5 попыток, завершаем работу.")
	}
	defer dbPool.Close()

	// Chromedp контекст
	opts := chromedp.DefaultExecAllocatorOptions[:]
	ctx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAllocator()

	browserCtx, cancelBrowser := chromedp.NewContext(ctx)
	defer cancelBrowser()

	// Канал для graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем главный цикл в отдельной горутине
	doneChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopChan:
				zap.L().Info("Получили сигнал на остановку, выходим из цикла парсинга.")
				close(doneChan)
				return
			default:
				// processIteration c ретраями при ошибках
				err := runIterationWithRetries(browserCtx, cfg, dbPool)
				if err != nil {
					zap.L().Error("Ошибка при итерации, прекращаем работу", zap.Error(err))
					// Если это фатальная ошибка (например, динамический хост не найден), выходим
					close(doneChan)
					return
				}
				// Задержка 200мс (или интеллектуальная логика)
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()

	// Ждём, пока цикл не завершится
	<-doneChan
	zap.L().Info("Парсер завершил работу.")
}

// runIterationWithRetries — если ошибка FetchData или капчи, пробуем заново
func runIterationWithRetries(ctx context.Context, cfg *Config, dbPool *pgxpool.Pool) error {
	const maxRetries = 3

	for i := 1; i <= maxRetries; i++ {
		err := processIteration(ctx, cfg, dbPool)
		if err == nil {
			return nil // успех
		}

		zap.L().Warn("Ошибка в processIteration", zap.Error(err), zap.Int("retry", i))

		// Если в error есть "динамический хост не найден" — завершаем немедленно
		if err.Error() == "динамический хост не найден" {
			return err // пусть внешний код фатально завершит
		}

		// Если ошибка капчи (сами решаем критерий), можно немедленно ретраить или ждать
		// time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("превышено число ретраев (%d), не удалось завершить итерацию", maxRetries)
}

// processIteration — один проход (решаем капчу, получаем HTML, парсим, сохраняем)
func processIteration(ctx context.Context, cfg *Config, dbPool *pgxpool.Pool) error {
	// 1. Решаем капчу
	err := SolveCaptcha(ctx, cfg)
	if err != nil {
		// Если капча есть, но мы не смогли её решить — возвращаем ошибку
		// => runIterationWithRetries попробует ещё раз
		return err
	}

	// 2. HTML
	htmlContent, err := GetHTML(ctx, cfg.LiveFootballURL)
	if err != nil {
		return err
	}

	// 3. Динамический хост
	dynamicHost, err := GetDynamicHost(htmlContent)
	if err != nil {
		// Если не нашли хост — завершаем
		return err
	}

	apiURL := "https://" + dynamicHost + "/events/list?lang=ru&scopeMarket=1600"

	// 4. Достаём данные из API
	data, err := FetchData(apiURL)
	if err != nil {
		return err
	}

	// 5. Сохраняем в БД
	err = SaveDataToPostgres(dbPool, *data)
	if err != nil {
		return err
	}

	zap.L().Debug("Итерация завершена")
	return nil
}
