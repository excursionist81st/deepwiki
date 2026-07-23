package main

import (
	"log"

	"deepseek_wiki/config"
	"deepseek_wiki/dao"
	"deepseek_wiki/routers"

	"github.com/gin-gonic/gin"
)

func main() {
	if err := config.Init(); err != nil {
		log.Fatal(err)
	}

	if err := dao.InitDB(); err != nil {
		log.Fatal("数据库初始化失败: ", err)
	}

	r := gin.Default()

	r.Static("/static", "./frontend")
	r.GET("/", func(c *gin.Context) {
		c.File("./frontend/index.html")
	})

	routers.SetupRoutes(r)

	r.Run(":8000")
}
