package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

// SolveCaptcha — главный метод для решения капчи (если она есть) на live-странице
func SolveCaptcha(ctx context.Context, cfg *Config) error {
	captchaURL, err := getCaptchaURL(ctx, cfg.LiveFootballURL)
	if err != nil {
		return fmt.Errorf("ошибка извлечения URL капчи: %w", err)
	}
	if captchaURL == "" {
		// Если возвращаем пустую строку — возможно, капчи нет.
		// Возвращаем nil, чтобы продолжать.
		zap.L().Debug("Капча не обнаружена на странице")
		return nil
	}

	captchaFile, err := downloadCaptchaImage(captchaURL)
	if err != nil {
		return fmt.Errorf("ошибка скачивания капчи: %w", err)
	}
	defer os.Remove(captchaFile) // удаляем файл после использования

	fileBytes, err := os.ReadFile(captchaFile)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла капчи: %w", err)
	}

	encodedCaptcha := base64.StdEncoding.EncodeToString(fileBytes)

	client := resty.New()

	// Создание задачи в Anti-Captcha
	res, err := client.R().
		SetBody(map[string]interface{}{
			"clientKey": cfg.AntiCaptchaAPIKey,
			"task": map[string]interface{}{
				"type": "ImageToTextTask",
				"body": encodedCaptcha,
			},
		}).
		Post("https://api.anti-captcha.com/createTask")
	if err != nil {
		return fmt.Errorf("ошибка отправки капчи в Anti-Captcha: %w", err)
	}

	zap.L().Info("Ответ от Anti-Captcha (создание задачи)",
		zap.ByteString("response", res.Body()),
	)

	var taskResponse struct {
		ErrorId int    `json:"errorId"`
		TaskId  int    `json:"taskId"`
		Message string `json:"errorDescription"`
	}
	err = json.Unmarshal(res.Body(), &taskResponse)
	if err != nil {
		return fmt.Errorf("ошибка разбора ответа Anti-Captcha: %w", err)
	}
	if taskResponse.ErrorId != 0 {
		return fmt.Errorf("Anti-Captcha ошибка: %s", taskResponse.Message)
	}

	// Ожидаем решения капчи
	var captchaText string
	for {
		time.Sleep(1 * time.Second)
		res, err := client.R().
			SetBody(map[string]interface{}{
				"clientKey": cfg.AntiCaptchaAPIKey,
				"taskId":    taskResponse.TaskId,
			}).
			Post("https://api.anti-captcha.com/getTaskResult")
		if err != nil {
			return fmt.Errorf("ошибка получения результата капчи: %w", err)
		}

		var resultResponse struct {
			Status   string `json:"status"`
			Solution struct {
				Text string `json:"text"`
			} `json:"solution"`
		}
		err = json.Unmarshal(res.Body(), &resultResponse)
		if err != nil {
			return fmt.Errorf("ошибка разбора результата Anti-Captcha: %w", err)
		}

		if resultResponse.Status == "ready" {
			captchaText = resultResponse.Solution.Text
			zap.L().Info("Капча успешно решена", zap.String("solution", captchaText))
			break
		}
	}

	// Ввести решение капчи на странице
	err = submitCaptchaSolution(ctx, captchaText)
	if err != nil {
		return fmt.Errorf("не удалось ввести решение капчи: %w", err)
	}

	return nil
}

// getCaptchaURL — ищем капчу на заданном URL
func getCaptchaURL(ctx context.Context, pageURL string) (string, error) {
	var captchaURL string

	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		// Подождём, вдруг капчи нет — ставим явный таймаут через chromedp.ActionFunc или контекст
		chromedp.WaitReady("body"),
		// Пробуем найти #captcha_image
		chromedp.AttributeValue(`#captcha_image`, "src", &captchaURL, nil),
	)
	if err != nil {
		// Если элемент не найден, вернём пустую строку
		zap.L().Debug("Капча не найдена (ошибка chromedp)", zap.Error(err))
		return "", nil
	}

	// Если captchaURL пуст, значит капчи нет
	if captchaURL == "" {
		return "", nil
	}

	fullURL := "https://fon.bet/" + captchaURL
	zap.L().Info("URL капчи успешно извлечён", zap.String("captcha_url", fullURL))
	return fullURL, nil
}

// submitCaptchaSolution — вводим капчу в форме
func submitCaptchaSolution(ctx context.Context, solution string) error {
	return chromedp.Run(ctx,
		chromedp.SetValue(`#captcha_input`, solution, chromedp.ByID),
		chromedp.Click(`#captcha_submit`, chromedp.ByID),
		chromedp.WaitNotPresent(`#captcha_image`, chromedp.ByID),
	)
}

func downloadCaptchaImage(captchaURL string) (string, error) {
	filePath := "captcha.png"

	resp, err := http.Get(captchaURL)
	if err != nil {
		return "", fmt.Errorf("ошибка загрузки изображения капчи: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка загрузки капчи, статус код: %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("ошибка создания файла капчи: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка записи файла капчи: %w", err)
	}

	if written < 100 {
		return "", fmt.Errorf("размер файла капчи слишком мал: %d байт", written)
	}

	zap.L().Info("Капча сохранена во временный файл",
		zap.String("file", filePath),
		zap.Int64("size", written),
	)
	return filePath, nil
}
