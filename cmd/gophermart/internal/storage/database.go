package storage

import (
	"crypto/md5"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/stdlib"
	_ "github.com/mattn/go-sqlite3"
	//	"github.com/lib/pq"
)

//	Database - структура хранилища данных, обертывающая пул подключений к базе данных
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
