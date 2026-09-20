package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func Init() error {
	if err := loadEnvFiles(); err != nil {
		return err
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			log.Println("[config] 未找到 config.yaml，完全使用环境变量")
		default:
			return fmt.Errorf("配置文件语法错误: %w", err)
		}
	} else {
		log.Printf("[config] 已加载 config.yaml: %s", viper.ConfigFileUsed())
	}

	return nil
}

func loadEnvFiles() error {
	envPath := os.Getenv("DEEPWIKI_ENV")
	if envPath == "" {
		envPath = `D:\env\deepwiki.env`
	}

	if err := godotenv.Load(envPath); err != nil {
		if os.IsNotExist(err) {
			log.Printf("[config] 外部 env 不存在: %s，降级加载当前目录 .env", envPath)
			if err := godotenv.Load(); err != nil {
				if !os.IsNotExist(err) {
					return fmt.Errorf("加载 .env 文件失败: %w", err)
				}
				log.Println("[config] 未找到任何 env 文件，仅使用系统环境变量")
			} else {
				log.Println("[config] 已加载当前目录 .env")
			}
		} else {
			return fmt.Errorf("加载外部 env 文件失败 (%s): %w", envPath, err)
		}
	} else {
		log.Printf("[config] 已加载外部 env: %s", envPath)
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
