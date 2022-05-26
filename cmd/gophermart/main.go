package main

import (
	"net/http"
	"time"

	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/handlers"
	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
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
		ErrorLog:   cfg.ErrorLog, //	журнал ошибок
		InfoLog:    cfg.InfoLog,  //	журнал информационных сообщений
		Datasource: datasource,   //	источник данных для сервера
	}

	//	запускаем процесс процесс синхронизации информации о заказах с внешней системой рассчёта баллов
	go func() {
		syncTicker := time.NewTicker(10 * time.Second) //	тикер для выдачи сигналов на синхронизацию
		defer syncTicker.Stop()
		for {
			<-syncTicker.C
			err := app.Datasource.UpdateOrdersStatus(cfg.AccrualAddress)
			if err != nil {
				cfg.ErrorLog.Println(err.Error())
			}
		}
	}()

	//	при остановке сервера закроем все источники данных
	defer app.Datasource.Close()

	//	запуск сервера
	cfg.InfoLog.Printf("Server starts at address: %s", cfg.ServerAddress)

	srv := &http.Server{
		Addr:     cfg.ServerAddress,
		ErrorLog: cfg.ErrorLog,
		Handler:  app.Routes(),
	}
	cfg.ErrorLog.Fatal(srv.ListenAndServe())
}
