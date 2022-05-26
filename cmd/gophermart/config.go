package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type Config struct {
	ServerAddress  string
	DatabaseDSN    string
	AccrualAddress string
	InfoLog        *log.Logger
	ErrorLog       *log.Logger
}

//	newConfig - функция-конфигуратор приложения через считывание флагов и переменных окружения
func newConfig() (cfg Config) {
	//	Приоритеты настроек:
	//	1.	Переменные окружения - ENV
	//	2.	Значения, задаваемые флагами при запуске из консоли
	//	3.	Значения по умолчанию.

	//	Считываем флаги запуска из командной строки и задаём значения по умолчанию, если флаг при запуске не указан
	ServerAddress := flag.String("a", "127.0.0.1:8080", "RUN_ADDRESS - адрес запуска сервера")
	DatabaseDSN := flag.String("d", "", "DATABASE_URI - адрес подключения к БД (PostgreSQL)")
	AccrualAddress := flag.String("r", "", "ACCRUAL_SYSTEM_ADDRESS - адрес доступа к системе расчёта начислений")
	//	парсим флаги
	flag.Parse()

	//	считываем переменные окружения
	//	если они заданы - переопределяем соответствующие локальные переменные:
	if u, flg := os.LookupEnv("RUN_ADDRESS"); flg {
		*ServerAddress = u
	}
	if u, flg := os.LookupEnv("DATABASE_URI"); flg {
		*DatabaseDSN = u
	}
	if u, flg := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS"); flg {
		*AccrualAddress = u
	}

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)                  // logger для информационных сообщений
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile) // logger для сообщений об ошибках

	// сигнальный канал для отслеживания системных вызовов на остановку сервера
	signalChanel := make(chan os.Signal, 1)
	signal.Notify(signalChanel,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	//	запускаем процесс слежение за сигналами на останов сервера
	go func() {
		for {
			s := <-signalChanel
			if s == syscall.SIGINT || s == syscall.SIGTERM || s == syscall.SIGQUIT {
				cfg.InfoLog.Println("SERVER Gophermart SHUTDOWN (code 0)")
				os.Exit(0)
			}
		}
	}()

	//	собираем конфигурацию сервера
	cfg = Config{
		ServerAddress:  *ServerAddress,
		DatabaseDSN:    *DatabaseDSN,
		AccrualAddress: *AccrualAddress,
		InfoLog:        infoLog,
		ErrorLog:       errorLog,
	}

	//	выводим в лог конфигурацию сервера
	log.Println("SERVER gophermart STARTED with configuration:\n   RUN_ADDRESS: ", cfg.ServerAddress, "\n   DATABASE_DSN: ", cfg.DatabaseDSN, "\n   ACCRUAL_SYSTEM_ADDRESS: ", cfg.AccrualAddress)

	return cfg
}
