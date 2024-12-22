package main

import (
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config — конечная структура для хранения настроек
type Config struct {
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresHost     string
	PostgresPort     int

	AntiCaptchaAPIKey string
	LiveFootballURL   string
}

// LoadConfig — читает cfg.yml (и ENV, если делать BindEnv) через viper
func LoadConfig() (*Config, error) {
	viper.SetConfigName("cfg") // файл называется cfg.yml
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// Пробуем прочитать cfg.yml
	if err := viper.ReadInConfig(); err != nil {
		zap.L().Fatal("Не удалось прочитать cfg.yml", zap.Error(err))
	}

	// Если хотим, чтобы ENV переопределяли YAML, тут вызываем BindEnv(...)
	// viper.BindEnv("postgres.user", "POSTGRES_USER")
	// и т.д. — при необходимости.

	// Считываем нужные поля
	port := viper.GetInt("postgres.port")
	if port == 0 {
		zap.L().Fatal("POSTGRES_PORT не указан ни в cfg.yml, ни в ENV (или =0)")
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

	// Проверяем, что критичные поля не пусты
	if cfg.PostgresUser == "" {
		return nil, fmt.Errorf("не задан postgres.user")
	}
	if cfg.PostgresPassword == "" {
		return nil, fmt.Errorf("не задан postgres.password")
	}
	if cfg.PostgresDB == "" {
		return nil, fmt.Errorf("не задан postgres.db")
	}
	if cfg.PostgresHost == "" {
		return nil, fmt.Errorf("не задан postgres.host")
	}
	if cfg.AntiCaptchaAPIKey == "" {
		return nil, fmt.Errorf("не задан anticaptcha.api_key")
	}
	if cfg.LiveFootballURL == "" {
		return nil, fmt.Errorf("не задан live_football_url")
	}

	zap.L().Info("Конфигурация загружена",
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
