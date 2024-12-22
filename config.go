package main

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct {
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresHost     string
	PostgresPort     int

	AntiCaptchaAPIKey string
	LiveFootballURL   string
}

func LoadConfig() (*Config, error) {
	// 1) Настраиваем Viper для чтения YAML-файла cfg.yml
	viper.SetConfigName("cfg")  // имя файла (без .yml)
	viper.SetConfigType("yaml") // или "yml" — Viper понимает YAML-формат одинаково
	viper.AddConfigPath(".")    // искать cfg.yml в текущей директории

	// 2) Пытаемся прочитать файл
	if err := viper.ReadInConfig(); err != nil {
		// Если файла нет или он пустой — считаем это критической ошибкой
		zap.L().Fatal("Не удалось прочитать cfg.yml. Проверьте, что файл существует и заполнен",
			zap.Error(err),
		)
	}

	// 3) Привязка к переменным окружения (если хотим, чтобы ENV переопределяли значения)
	viper.BindEnv("postgres.user", "POSTGRES_USER")
	viper.BindEnv("postgres.password", "POSTGRES_PASSWORD")
	viper.BindEnv("postgres.db", "POSTGRES_DB")
	viper.BindEnv("postgres.host", "POSTGRES_HOST")
	viper.BindEnv("postgres.port", "POSTGRES_PORT")

	viper.BindEnv("anticaptcha.api_key", "ANTICAPTCHA_APIKEY")
	viper.BindEnv("live_football_url", "LIVE_FOOTBALL_URL")

	// 4) Получаем значения (не ставим дефолты — если чего-то нет, будет пусто или 0)
	port := viper.GetInt("postgres.port")
	if port == 0 {
		// Если в файле и ENV нет значения, или оно 0 — считаем ошибкой
		zap.L().Fatal("POSTGRES_PORT не указан ни в cfg.yml, ни в переменных окружения, либо 0")
	}

	cfg := &Config{
		PostgresUser:     viper.GetString("postgres.user"),
		PostgresPassword: viper.GetString("postgres.password"),
		PostgresDB:       viper.GetString("postgres.db"),
		PostgresHost:     viper.GetString("postgres.host"),
		PostgresPort:     port,

		AntiCaptchaAPIKey: viper.GetString("anticaptcha.api_key"),
		LiveFootballURL:   viper.GetString("live_football_url"),
	}

	// Дополнительные проверки (например, чтобы поля не были пустыми)
	if cfg.PostgresUser == "" || cfg.PostgresPassword == "" || cfg.PostgresDB == "" ||
		cfg.PostgresHost == "" || cfg.AntiCaptchaAPIKey == "" || cfg.LiveFootballURL == "" {
		return nil, fmt.Errorf("не все поля заполнены в cfg.yml или окружении")
	}

	// Логируем результат (скрыв пароли/ключи)
	zap.L().Info("Конфигурация загружена без дефолтов",
		zap.String("PostgresUser", cfg.PostgresUser),
		zap.String("PostgresPassword", "[HIDDEN]"),
		zap.String("PostgresDB", cfg.PostgresDB),
		zap.String("PostgresHost", cfg.PostgresHost),
		zap.Int("PostgresPort", cfg.PostgresPort),
		zap.String("AntiCaptchaAPIKey", "[HIDDEN]"),
		zap.String("LiveFootballURL", cfg.LiveFootballURL),
	)

	return cfg, nil
}
