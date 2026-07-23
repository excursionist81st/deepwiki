package config

import (
	"fmt"

	"github.com/spf13/viper"
)

func Init() error {
	viper.AutomaticEnv()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			return fmt.Errorf("配置文件不存在: ./config/config.yaml")
		default:
			return fmt.Errorf("配置文件语法错误: %w", err)
		}
	}

	return nil
}

func GetString(key string) string {
	value := viper.GetString(key)
	if value == "" {
		panic(fmt.Sprintf("%s 未配置", key))
	}
	return value
}

func GetInt(key string) int {
	value := viper.GetInt(key)
	if value == 0 {
		panic(fmt.Sprintf("%s 未配置", key))
	}
	return value
}
