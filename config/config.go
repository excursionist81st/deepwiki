package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func Init() error {
	// 加载 .env 文件（可选）
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("加载 .env 文件失败: %w", err)
		}
		// .env 文件不存在，使用系统环境变量
	}

	viper.AutomaticEnv()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			// 配置文件可选，使用环境变量
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

func GetFloat(key string) float64 {
	return viper.GetFloat64(key)
}
