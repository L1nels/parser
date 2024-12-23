package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"time"

	_ "golang.org/x/image/bmp"

	_ "golang.org/x/image/webp" // Исправленный импорт

	"github.com/chromedp/chromedp"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

// SolveCaptcha — решаем капчу (блокируемся, пока не решим)
func SolveCaptcha(ctx context.Context, cfg *Config) error {
	captchaURL, err := getCaptchaURL(ctx, cfg.LiveFootballURL)
	if err != nil {
		return fmt.Errorf("ошибка извлечения URL капчи: %w", err)
	}
	if captchaURL == "" {
		zap.L().Debug("Капча не найдена (captchaURL пуст)")
		return nil
	}

	// Вместо http.Get(...) — делаем скрин элемента
	captchaFile, err := downloadCaptchaImage(ctx)
	if err != nil {
		return fmt.Errorf("ошибка скачивания капчи: %w", err)
	}
	defer os.Remove(captchaFile)

	fileBytes, err := os.ReadFile(captchaFile)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла капчи: %w", err)
	}
	encodedCaptcha := base64.StdEncoding.EncodeToString(fileBytes)

	// Отправляем в Anti-Captcha
	client := resty.New()
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
		return fmt.Errorf("ошибка отправки капчи: %w", err)
	}

	zap.L().Info("Ответ от Anti-Captcha (создание задачи)",
		zap.ByteString("response", res.Body()),
	)

	var taskResponse struct {
		ErrorId int    `json:"errorId"`
		TaskId  int    `json:"taskId"`
		Message string `json:"errorDescription"`
	}
	if err := json.Unmarshal(res.Body(), &taskResponse); err != nil {
		return fmt.Errorf("ошибка разбора ответа Anti-Captcha: %w", err)
	}
	if taskResponse.ErrorId != 0 {
		return fmt.Errorf("Anti-Captcha ошибка: %s", taskResponse.Message)
	}

	// Ждём решения
	var captchaText string
	for {
		time.Sleep(1 * time.Second)
		r, err := client.R().
			SetBody(map[string]interface{}{
				"clientKey": cfg.AntiCaptchaAPIKey,
				"taskId":    taskResponse.TaskId,
			}).
			Post("https://api.anti-captcha.com/getTaskResult")
		if err != nil {
			return fmt.Errorf("ошибка получения результата капчи: %w", err)
		}

		var result struct {
			Status   string `json:"status"`
			Solution struct {
				Text string `json:"text"`
			} `json:"solution"`
		}
		if err := json.Unmarshal(r.Body(), &result); err != nil {
			return fmt.Errorf("ошибка разбора результата Anti-Captcha: %w", err)
		}

		if result.Status == "ready" {
			zap.L().Info("Капча успешно решена", zap.String("solution", result.Solution.Text))
			captchaText = result.Solution.Text
			break
		}
	}

	// Вводим решение (если действительно нужно)
	if err := submitCaptchaSolution(ctx, captchaText); err != nil {
		return fmt.Errorf("не удалось ввести решение капчи: %w", err)
	}
	return nil
}

// getCaptchaURL — ищем #captcha_image
func getCaptchaURL(ctx context.Context, pageURL string) (string, error) {
	var captchaURL string
	err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
		chromedp.AttributeValue(`#captcha_image`, "src", &captchaURL, nil),
	)
	if err != nil {
		zap.L().Debug("Вероятно, капчи нет (chromedp ошибка)", zap.Error(err))
		return "", nil
	}
	if captchaURL == "" {
		return "", nil
	}
	fullURL := "https://fon.bet/" + captchaURL
	zap.L().Info("URL капчи успешно извлечён", zap.String("captcha_url", fullURL))
	return fullURL, nil
}

// submitCaptchaSolution — примерный ввод
func submitCaptchaSolution(ctx context.Context, solution string) error {
	return chromedp.Run(ctx,
		chromedp.SetValue(`#captcha_input`, solution, chromedp.ByID),
		chromedp.Click(`#captcha_submit`, chromedp.ByID),
		chromedp.WaitNotPresent(`#captcha_image`, chromedp.ByID),
	)
}

// downloadCaptchaImage — делаем скриншот #captcha_image
func downloadCaptchaImage(ctx context.Context) (string, error) {
	var buf []byte

	// Шаг 1: делаем скрин элемента #captcha_image
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`#captcha_image`, chromedp.ByID),
		chromedp.Screenshot(`#captcha_image`, &buf, chromedp.NodeVisible, chromedp.ByID),
	); err != nil {
		return "", fmt.Errorf("ошибка скриншота капчи: %w", err)
	}

	// Шаг 2: Декодируем (проверка, что это реальный PNG)
	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("ошибка декодирования скриншота: %w", err)
	}

	// Шаг 3: Сохраняем в captcha.png
	filePath := "captcha.png"
	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("ошибка создания файла капчи: %w", err)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		return "", fmt.Errorf("ошибка сохранения PNG: %w", err)
	}
	stat, _ := out.Stat()
	zap.L().Info("Скрин капчи сохранён",
		zap.String("file", filePath),
		zap.Int64("size", stat.Size()),
	)
	return filePath, nil
}
