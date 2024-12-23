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
	viper.SetConfigName("cfg") // ищет файл cfg.yml
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		zap.L().Fatal("Не удалось прочитать cfg.yml", zap.Error(err))
	}

	port := viper.GetInt("postgres.port")
	if port == 0 {
		zap.L().Fatal("POSTGRES_PORT не указан или 0")
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

	// Проверяем, что все поля заполнены
	if cfg.PostgresUser == "" || cfg.PostgresPassword == "" || cfg.PostgresDB == "" ||
		cfg.PostgresHost == "" || cfg.AntiCaptchaAPIKey == "" || cfg.LiveFootballURL == "" {
		return nil, fmt.Errorf("не все поля заданы в cfg.yml")
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
