package main

import (
	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/handlers"
	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
	"net/http"
)

func main() {
	//	конфигурация приложения через считывание флагов и переменных окружения
	cfg := newConfig()

	//	инициализируем источники данных нашего сервера
	datasource, err := storage.NewDatasource(cfg.DatabaseDSN)
	if err != nil {
		cfg.ErrorLog.Fatal(err)
	}

	//	инициализируем контекст нашего приложения
	app := &handlers.Application{
		ErrorLog:       cfg.ErrorLog,       //	журнал ошибок
		InfoLog:        cfg.InfoLog,        //	журнал информационных сообщений
		Datasource:     datasource,         //	источник данных для сервера
		AccrualAddress: cfg.AccrualAddress, //	адрес сервиса расчёта бонусных баллов
	}

	//	запускаем процесс синхронизации информации о заказах с внешней системой расчёта баллов
	go syncer(app)

	//	запускаем процесс слежение за сигналами на останов сервера
	go termSignal()

	//	при остановке сервера закроем все источники данных
	defer app.Datasource.Close()

	//	запуск сервер
	srv := &http.Server{
		Addr:     cfg.ServerAddress,
		ErrorLog: cfg.ErrorLog,
		Handler:  app.Routes(),
	}
	cfg.ErrorLog.Fatal(srv.ListenAndServe())
}
