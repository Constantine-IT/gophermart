package main

import (
	"context"
	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/handlers"
	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	//	конфигурация приложения через считывание флагов и переменных окружения
	cfg := newConfig()

	//	инициализируем источники данных нашего сервера
	datasource, err := storage.NewDatasource(cfg.DatabaseDSN, cfg.AccrualAddress)
	if err != nil {
		cfg.ErrorLog.Fatal(err)
	}
	defer datasource.Close() //	при остановке сервера закроем все источники данных

	//	инициализируем контекст нашего приложения
	app := &handlers.Application{
		ErrorLog:   cfg.ErrorLog, //	журнал ошибок
		InfoLog:    cfg.InfoLog,  //	журнал информационных сообщений
		Datasource: datasource,   //	источник данных для хранения информации о заказах
	}

	//	создаем контекст для остановки служебных процессов по сигналу
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//	запускаем процесс синхронизации информации о заказах с внешней системой расчёта баллов
	go statusSyncer(app, ctx)

	//	запускаем процесс слежение за сигналами на останов сервера
	go termSignal(cancel)

	//	запуск сервера
	srv := &http.Server{
		Addr:     cfg.ServerAddress,
		ErrorLog: cfg.ErrorLog,
		Handler:  app.Routes(),
	}
	cfg.ErrorLog.Fatal(srv.ListenAndServe())
}

//	 statusSyncer - синхронизатор информации о заказах с внешней системой расчёта баллов
func statusSyncer(app *handlers.Application, ctx context.Context) {
	syncTicker := time.NewTicker(10 * time.Second) //	тикер для выдачи сигналов на синхронизацию
	defer syncTicker.Stop()
	for { //	вызываем обновление статусов для заказов, находящихся у нас в базе НЕ в финальных статусах
		err := app.Datasource.UpdateOrdersStatus()

		if err != nil {
			app.ErrorLog.Println(err.Error()) //	все ошибки пишем в журнал
		}

		select {
		case <-syncTicker.C: //	повторяем обновление статусов на каждое срабатывание тикера

		case <-ctx.Done(): //	при подаче сигнала на останов сервера, прерываем процесс синхронизации
			app.InfoLog.Println("Synchronization with bonus server stopped")
			return
		}
	}
}

// termSignal - функция слежения за сигналами на останов сервера
func termSignal(cancel context.CancelFunc) {
	// сигнальный канал для отслеживания системных вызовов на остановку сервера
	signalChanel := make(chan os.Signal, 1)
	signal.Notify(signalChanel,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	//	запускаем слежение за каналом
	for {
		s := <-signalChanel
		if s == syscall.SIGINT || s == syscall.SIGTERM || s == syscall.SIGQUIT {
			cancel()
			time.Sleep(1 * time.Second)
			log.Println("SERVER Gophermart SHUTDOWN (code 0)")
			os.Exit(0) //	при получении сигнала, останавливаем сервер
		}
	}
}
