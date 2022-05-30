package handlers

import (
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Constantine-IT/gophermart/cmd/gophermart/internal/storage"
)

type Application struct {
	ErrorLog       *log.Logger        //	журнал ошибок
	InfoLog        *log.Logger        //	журнал информационных сообщений
	Datasource     storage.Datasource //	источник данных для хранения информации о заказах
	AccrualAddress string             //	адрес сервиса расчёта бонусных баллов
}

func (app *Application) Routes() chi.Router {

	// определяем роутер chi
	r := chi.NewRouter()

	// зададим middleware для поддержки компрессии тел запросов и ответов
	r.Use(middleware.Compress(1, `text/plain`, `application/json`))
	r.Use(middleware.AllowContentEncoding(`gzip`))
	r.Use(app.DecompressGZIP)

	// зададим встроенные middleware, чтобы улучшить стабильность приложения
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	//r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	//	маршруты сервера и их обработчики
	r.Route("/", func(r chi.Router) {
		r.Post("/api/user/register", app.UserRegistrationHandler)
		r.Post("/api/user/login", app.UserAuthenticationHandler)
		r.Post("/api/user/orders", app.PostUserOrderHandler)
		r.Post("/api/user/balance/withdraw", app.PostWithdrawRequestHandler)
		r.Get("/api/user/orders", app.GetUserOrdersHandler)
		r.Get("/api/user/balance", app.GetUserBalanceHandler)
		r.Get("/api/user/balance/withdrawals", app.GetUserWithdrawalsHandler)
	})

	return r
}

/*
Сводное HTTP API накопительной системы лояльности:

POST /api/user/register — регистрация пользователя;
POST /api/user/login — аутентификация пользователя;
POST /api/user/orders — загрузка пользователем номера заказа для расчёта;
GET /api/user/orders — получение списка загруженных пользователем номеров заказов, статусов их обработки и информации о начислениях;
GET /api/user/balance — получение текущего баланса счёта баллов лояльности пользователя;
POST /api/user/balance/withdraw — запрос на списание баллов с накопительного счёта в счёт оплаты нового заказа;
GET /api/user/balance/withdrawals — получение информации о выводе средств с накопительного счёта пользователем.
*/
