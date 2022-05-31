package main

import (
	"flag"
	"log"
	"os"
)

//	Config - структура хранения конфигурации нашего сервера
type Config struct {
	ServerAddress  string      //	адрес запуска сервера
	DatabaseDSN    string      //	адрес подключения к БД (PostgreSQL)
	AccrualAddress string      //	адрес доступа к системе расчёта начислений
	InfoLog        *log.Logger //	logger для информационных сообщений
	ErrorLog       *log.Logger //	logger для сообщений об ошибках
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

	//	собираем конфигурацию сервера
	cfg = Config{
		ServerAddress:  *ServerAddress,
		DatabaseDSN:    *DatabaseDSN,
		AccrualAddress: *AccrualAddress,
		InfoLog:        infoLog,
		ErrorLog:       errorLog,
	}

	//	выводим в лог конфигурацию сервера
	log.Println("SERVER Gophermart STARTED with configuration:\n   RUN_ADDRESS: ", cfg.ServerAddress, "\n   DATABASE_DSN: ", cfg.DatabaseDSN, "\n   ACCRUAL_SYSTEM_ADDRESS: ", cfg.AccrualAddress)

	return cfg
}
