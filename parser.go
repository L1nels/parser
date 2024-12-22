package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

// GetDynamicHost — ищем lineXX.bk6bba-resources.com
func GetDynamicHost(html string) (string, error) {
	re := regexp.MustCompile(`https://(line\d+w\.bk6bba-resources\.com)`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", errors.New("динамический хост не найден")
}

// GetHTML — загружаем весь HTML страницы
func GetHTML(ctx context.Context, url string) (string, error) {
	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("html"),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return "", fmt.Errorf("ошибка chromedp.Run: %w", err)
	}
	return html, nil
}

// FetchData — запрос к API, парсим JSON в ApiResponse
func FetchData(apiURL string) (*ApiResponse, error) {
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		zap.L().Error("Ошибка ответа API", zap.Int("status_code", resp.StatusCode))
		return nil, fmt.Errorf("ошибка ответа API, статус %d", resp.StatusCode)
	}

	var result ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ошибка декодирования JSON: %w", err)
	}

	zap.L().Info("Данные от API получены",
		zap.Int("events_count", len(result.Events)),
	)
	return &result, nil
}
