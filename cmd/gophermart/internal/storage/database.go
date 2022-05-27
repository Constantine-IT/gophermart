package storage

import (
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/jackc/pgx/stdlib"
	_ "github.com/mattn/go-sqlite3"
	//	"github.com/lib/pq"

	"github.com/go-resty/resty/v2"
)

//	Database - структура хранилища данных, обертывающая пул подключений к базе данных PostgreSQL
//	реализует интерфейс Datasource
type Database struct {
	DB *sql.DB
}

//	UserRegister - метод создания нового пользователя в системе лояльности
func (d *Database) UserRegister(userID, password string) (token string, err error) {
	//	пустые значения password или UserID к вставке в хранилище не допускаются
	if userID == "" || password == "" {
		return "", ErrEmptyNotAllowed
	}

	// проверяем, есть ли пользователь с таким login в нашей базе
	var userIDfromDB string
	stmt := `select "userid" from "users" where "userid" = $1`
	err = d.DB.QueryRow(stmt, userID).Scan(&userIDfromDB)
	if !errors.Is(err, sql.ErrNoRows) { //	если в базе уже есть пользователь с таким login
		return "", ErrUserAlreadyExist
	}

	//	если пользователя с таким login нет в нашей базе - начинаем тразакцию
	tx, err := d.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback() //	при ошибке выполнения - откатываем транзакцию

	//	готовим SQL-statement для вставки в базу нового пользователя
	stmtInsert, err := tx.Prepare(`insert into "users" ("userid", "password", "session_id") values ($1, $2, $3)`)
	if err != nil {
		return "", err
	}
	defer stmtInsert.Close()

	//	преобразуем комбинацию логин/пароль в hash - так и храним в базе из соображений безопасности
	mdSum := md5.Sum([]byte(userID + password + userID))
	hash := fmt.Sprintf("%x", mdSum)

	//	генерируем новый идентификатор сессии пользователя
	sessionID := newSessionID()
	//	 запускаем SQL-statement на исполнение
	if _, err := stmtInsert.Exec(userID, hash, sessionID); err != nil {
		return "", err
	}

	//	при успешном выполнении вставки - фиксируем транзакцию и возращаем идентификатор сесии
	return sessionID, tx.Commit()
}

//	UserAuthorise - метод авторизации пользователя в системе лояльности
func (d *Database) UserAuthorise(userID, password string) (token string, err error) {

	//	пустые значения password или UserID не допускаются
	if userID == "" || password == "" {
		return "", ErrEmptyNotAllowed
	}

	// проверяем, есть ли пользователь с таким login в нашей базе
	var passwordFromDB string
	stmt := `select "password" from "users" where "userid" = $1`
	err = d.DB.QueryRow(stmt, userID).Scan(&passwordFromDB)

	if errors.Is(err, sql.ErrNoRows) { //	если запрос не вернул строк - в базе нет пользователя с таким login
		return "", ErrLoginPasswordIsWrong
	}
	if err != nil {
		return "", err
	}

	//	преобразуем комбинацию входящих логин/пароль в hash - как мы храним их в нашей базе из соображений безопасности
	mdSum := md5.Sum([]byte(userID + password + userID))
	hash := fmt.Sprintf("%x", mdSum)

	if passwordFromDB != hash { //	если hash пароля в базе не совпадает с hash присланного пароля
		return "", ErrLoginPasswordIsWrong
	}

	//	если логин/пароль совпали выдаём идентификатор сессии - начинаем тразакцию
	tx, err := d.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback() //	при ошибке выполнения - откатываем транзакцию

	//	готовим SQL-statement для обновления в базе информации об идентификаторе сессии
	stmtInsert, err := tx.Prepare(`update "users" set "session_id" = $1 where "userid" = $2`)
	if err != nil {
		return "", err
	}
	defer stmtInsert.Close()

	//	генерируем новый идентификатор сессии пользователя
	sessionID := newSessionID()
	//	 запускаем SQL-statement на исполнение
	if _, err := stmtInsert.Exec(sessionID, userID); err != nil {
		return "", err
	}

	//	при успешном выполнении обновления в базе - фиксируем транзакцию и возвращаем идентификатор сессии
	return sessionID, tx.Commit()
}

//	GetOrders - метод, который возвращает список всех заказов для начисления баллов на счёт данного пользователя
func (d *Database) GetOrders(sessionID string) ([]Order, error) {
	var orderNum string
	var accrual float32
	var status, processed string
	orders := make([]Order, 0)

	stmt := `select "order", "status", "accrual", "uploaded_at" from "orders", "users" where "orders"."userid" = "users"."userid" and "session_id" = $1 order by "uploaded_at"`
	rows, err := d.DB.Query(stmt, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoDataToAnswer
	}
	if err != nil || rows.Err() != nil {
		return nil, err
	}
	defer rows.Close()
	//	перебираем все строки выборки, добавляя записи withdraw в исходящий срез withdrawals
	for rows.Next() {
		err := rows.Scan(&orderNum, &status, &accrual, &processed)
		if err != nil {
			return nil, err
		}
		orders = append(orders, Order{Number: orderNum, Accrual: accrual, Status: status, UploadedAt: processed})
	}

	if len(orders) == 0 { //	если заказов на начисление баллов не было
		return nil, ErrNoDataToAnswer
	}

	return orders, nil
}

// GetBalance - метод, который возвращает все текущие начисления и списания пользователя
func (d *Database) GetBalance(sessionID string) (accrualSum, withdrawSum float32, err error) {

	// выбираем заказы пользователя в статусе PROCESSED и считаем по ним общую сумму начислений
	stmt := `select SUM("accrual") from "orders", "users" where "orders"."userid" = "users"."userid" and "session_id" = $1 and "status" = $2 group by "orders"."userid"`
	err = d.DB.QueryRow(stmt, sessionID, "PROCESSED").Scan(&accrualSum)
	if errors.Is(err, sql.ErrNoRows) {
		accrualSum = 0
	}

	// выбираем все списания пользователя за всё время
	stmt = `select SUM("sum") from "withdrawals", "users" where "withdrawals"."userid" = "users"."userid" and "session_id" = $1 group by "withdrawals"."userid"`
	err = d.DB.QueryRow(stmt, sessionID).Scan(&withdrawSum)
	if errors.Is(err, sql.ErrNoRows) {
		withdrawSum = 0
	}

	return accrualSum - withdrawSum, withdrawSum, nil
}

//	GetWithdrawals - метод, который возвращает список всех списаний баллов со счёта данного пользователя
func (d *Database) GetWithdrawals(sessionID string) ([]Withdraw, error) {
	var order string
	var sum float32
	var processed string
	withdrawals := make([]Withdraw, 0)

	stmt := `select "order", "sum", "processed_at" from "withdrawals", "users" where "withdrawals"."userid" = "users"."userid" and "session_id" = $1 order by "processed_at"`
	rows, err := d.DB.Query(stmt, sessionID)
	if err != nil || rows.Err() != nil {
		return nil, err
	}
	defer rows.Close()
	//	перебираем все строки выборки, добавляя записи withdraw в исходящий срез withdrawals
	for rows.Next() {
		err := rows.Scan(&order, &sum, &processed)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, Withdraw{Order: order, Sum: sum, ProcessedAt: processed})
	}

	if len(withdrawals) == 0 { //	если списаний не было
		return nil, ErrNoDataToAnswer
	}

	return withdrawals, nil
}

//	OrderInsert - метод вносящий новый заказ в список программы лояльности
func (d *Database) OrderInsert(order string, sessonID string) error {
	//	пустые значения order или sessonID к вставке в хранилище не допускаются
	if order == "" || sessonID == "" {
		return ErrEmptyNotAllowed
	}

	// проверяем, не содержится ли заказ уже в нашей базе
	var sessIDfromDB string
	stmt := `select "session_id" from "orders", "users" where "orders"."userid" = "users"."userid" and "order" = $1`
	err := d.DB.QueryRow(stmt, order).Scan(&sessIDfromDB)
	if !errors.Is(err, sql.ErrNoRows) { //	если в базе уже есть строка с таким номером заказа
		if sessIDfromDB == sessonID {
			return ErrOrderExistToAccount //	если заказ уже привязан к аккаунту этого пользователя
		} else {
			return ErrOrderExistToAnother //	если заказ уже привязан к аккаунту другого пользователя
		}
	}

	//	если такого заказа ещё нет в базе - начинаем тразакцию
	tx, err := d.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //	при ошибке выполнения - откатываем транзакцию

	//	готовим SQL-statement для вставки в базу нового заказа
	stmtInsert, err := tx.Prepare(`insert into "orders" ("order", "status", "accrual", "uploaded_at", "userid") values ($1, 'NEW', 0, $2, (select "userid" from "users" where "session_id" = $3))`)
	if err != nil {
		return err
	}
	defer stmtInsert.Close()

	//	 запускаем SQL-statement на исполнение
	if _, err := stmtInsert.Exec(order, time.Now().Format(time.RFC3339), sessonID); err != nil {
		return err
	}

	return tx.Commit() //	при успешном выполнении вставки - фиксируем транзакцию
}

//	WithdrawRequest - метод создаёт новую заявку на оплату заказа баллами программы лояльности
func (d *Database) WithdrawRequest(order string, sum float32, sessionID string) error {

	//	пустые значения order или UserID к вставке в хранилище не допускаются
	if order == "" || sum == 0 || sessionID == "" {
		return ErrEmptyNotAllowed
	}

	// проверяем, достаточно ли средств на балансе пользователя
	accrualSum, withdrawSum, errSum := d.GetBalance(sessionID)
	if errSum != nil {
		return errSum
	}
	if sum > accrualSum-withdrawSum {
		return ErrInsufficientFundsToAccount
	}

	//	если средств на счёте достаточно для списания по запросу - начинаем тразакцию
	tx, err := d.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //	при ошибке выполнения - откатываем транзакцию

	//	готовим SQL-statement для вставки в базу нового заказа
	stmt, err := tx.Prepare(`insert into "withdrawals" ("order", "sum", "processed_at", "userid") values ($1, $2, $3, (select "userid" from "users" where "session_id" = $4))`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	//	 запускаем SQL-statement на исполнение, в качестве даты вставляем текущее время в формате RFC3339
	if _, err := stmt.Exec(order, sum, time.Now().Format(time.RFC3339), sessionID); err != nil {
		return err
	}

	return tx.Commit() //	при успешном выполнении вставки - фиксируем транзакцию
}

//	Close - метод, закрывающий connect к базе данных
func (d *Database) Close() {
	//	при остановке сервера connect к базе данных
	d.DB.Close()
	time.Sleep(3 * time.Second)
}

//	UpdateOrdersStatus - метод обновления статусов заказов и начисленных баллов
//	при сверке с внешним сервисом рассчёта бонусных баллов
func (d *Database) UpdateOrdersStatus(AccrualAddress string) error {
	var orderNum string
	orders := make([]Order, 0)

	//	выбираем из базы заказы, находящиеся НЕ в финальных статусах - PROCESSED и INVALID
	//	у нас это статусы - NEW и PROCESSING
	stmt := `select "order" from "orders" where "orders"."status" = 'NEW' or "orders"."status" = 'PROCESSING'`
	rows, err := d.DB.Query(stmt)
	if err != nil || rows.Err() != nil {
		return err
	}
	defer rows.Close()
	//	перебираем все строки выборки, добавляя записи в список заказов на синхронизацию с системой начисления баллов
	for rows.Next() {
		err := rows.Scan(&orderNum)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}

		//	до синхронизации переводим все новые заказы в статус PROCESSING, с суммой начисленных баллов = 0
		//	и формируем из них список orders
		orders = append(orders, Order{Number: orderNum, Accrual: 0, Status: "PROCESSING"})
	}

	//	если заказов для синхронизации не нашлось - то завершаем на этом процесс синхронизации
	if len(orders) == 0 { //	если заказов на начисление баллов не было
		return nil
	}

	//	описываем структуру для приема данных о статусе заказа в JSON виде
	type ordersSync struct {
		Order   string  `json:"order"`
		Status  string  `json:"status"`
		Accrual float32 `json:"accrual,omitempty"`
	}
	//	создаём экземпляр этой структуры
	ordersUpdated := ordersSync{}

	//	создаём клиент HTTP для запросов о статусе заказа в систему начисления баллов
	client := resty.New()

	//	опрашиваем статус всех заказов из списка orders для получения их текущего статуса
	for i := range orders {
		//	для запросов в систему начисления баллов используется запрос:
		//	GET /api/orders/{number} — получение информации о расчёте начислений баллов лояльности
		resp, err := client.R().Get(AccrualAddress + "/api/orders/" + orders[i].Number)
		if err != nil {
			return err
		}

		status := resp.StatusCode() //	считываем код статуса ответа

		for status == http.StatusTooManyRequests { //	если пришел ответ со статусом 429 - TooManyRequests
			log.Println("response status - TooManyRequests for sync service")
			time.Sleep(5 * time.Second) //	если превышен лимит количества запросов в минуту, делаем паузу
			//	и повторяем запрос с теми же параметрами
			resp, err := client.R().Get(AccrualAddress + "/api/orders/" + orders[i].Number)
			if err != nil {
				return err
			}
			status = resp.StatusCode() //	считываем код статуса ответа
		}

		if status == http.StatusOK { //	если пришел ответ со статусом 200 - ОК

			body := resp.Body() //	считываем тело ответа
			//	парсим JSON и записываем результат в ordersUpdated
			errParsing := json.Unmarshal(body, &ordersUpdated)
			//	проверяем успешно ли парсится JSON
			if errParsing != nil {
				log.Println(errParsing.Error()) // запишем в лог сообщение об ошибке
				continue                        //	и продолжаем цикл в новой итерации
			}
			//	нас интересуют только заказы перешедшие в финальные статусы - PROCESSED и INVALID
			//	меняем в списке orders для них статус и сумму начислений - на актуальные значения
			if ordersUpdated.Status == "PROCESSED" || ordersUpdated.Status == "INVALID" {
				orders[i].Status = ordersUpdated.Status
				orders[i].Accrual = ordersUpdated.Accrual
			}
		} else {
			continue //	если произошла неизвестная ошибка - продолжаем цикл в новой итерации
		}
	}

	//	теперь в списке orders лежит обновленная информация по заказам на начисление баллов
	//	заносим эту информацию в нашу базу
	//	начинаем транзакцию
	tx, err := d.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //	при ошибке выполнения - откатываем транзакцию

	//	готовим SQL-statement для обновления в базе информации
	stmtInsert, err := tx.Prepare(`update "orders" set "status" = $1, "accrual" = $2 where "order" = $3`)
	if err != nil {
		return err
	}
	defer stmtInsert.Close()

	for i := range orders {
		//	 запускаем обновление для каждого элемента списка на исполнение
		if _, err := stmtInsert.Exec(orders[i].Status, orders[i].Accrual, orders[i].Number); err != nil {
			log.Println(err.Error()) //	если при вставке произошла ошибка, то заносим её в журнал
			continue                 //	 и продолжаем выполнение вставок далее по списку orders
		}
	}
	//	фиксируем транзакцию, и результат фиксации возвращаем в вызывающую функцию
	return tx.Commit()
}
