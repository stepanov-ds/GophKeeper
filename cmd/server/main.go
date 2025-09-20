package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/stepanov-ds/GophKeeper/internal/config"
	"github.com/stepanov-ds/GophKeeper/internal/database"
	"github.com/stepanov-ds/GophKeeper/internal/handlers/router"
)

func main() {
	//настройка логгера
	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)

	//конфигурация сервиса
	config.ConfigServer()

	//инициализация БД
	database.InitConnection()
	database.RunMigrations()

	//запуск сервера gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	router.Route(r)
	if err := r.Run(*config.EndpointServer); err != nil {
		log.Panicln(err)
	}

}
