package storage

import (
	"errors"
	"log"
	"net/http"
	"time"

	"database/sql"
	"encoding/json"
	"github.com/go-resty/resty/v2"
)

//	UpdateOrdersStatus - метод синхронизации статусов заказов и начисленных баллов с внешним сервисом расчёта бонусных баллов
func (d *Database) UpdateOrdersStatus(AccrualAddress string) error {

	//	выбираем из базы заказы, находящиеся в НЕ финальных статусах - NEW и PROCESSING
	stmt := `select "order", "uploaded_at" from "orders" where "orders"."status" = 'NEW' or "orders"."status" = 'PROCESSING'`

	rows, err := d.DB.Query(stmt) //	готовим и компилируем SQL-statement
	if err != nil || rows.Err() != nil {
		return err
	}
	defer rows.Close()

	var orderNum, uploadTime string
	orders := make([]Order, 0)

	for rows.Next() { //	перебираем все строки выборки
		err := rows.Scan(&orderNum, &uploadTime)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		//	и формируем из них список orders для синхронизации с системой начисления баллов
		orders = append(orders, Order{Number: orderNum, Accrual: 0, Status: "PROCESSING", UploadedAt: uploadTime})
		//	до синхронизации переводим все новые заказы в статус PROCESSING, с суммой начисленных баллов = 0
	}

	//	если заказов для синхронизации не нашлось - то завершаем на этом процесс синхронизации
	if len(orders) == 0 { //	если заказов на начисление баллов не было
		return nil
	}

	//	если заказы нашлись, то синхронизуем их статусы и начисления с сервером начисления бонусных баллов
	err = syncStatusWithBonusServer(orders, AccrualAddress)
	if err != nil {
		return err
	}

	//	теперь в списке orders лежит обновленная информация по заказам на начисление баллов - обновим нашу базу
	tx, err := d.DB.Begin() //	начинаем транзакцию
	if err != nil {
		return err
	}
	defer tx.Rollback() //	при ошибке выполнения - откатываем транзакцию

	//	готовим SQL-statement для обновления в базе информации по заказам
	stmtInsert, err := tx.Prepare(`update "orders" set "status" = $1, "accrual" = $2, "uploaded_at" = $3 where "order" = $4`)
	if err != nil {
		return err
	}
	defer stmtInsert.Close()

	for i := range orders { //	 запускаем обновление для каждого элемента списка на исполнение
		if _, err := stmtInsert.Exec(orders[i].Status, orders[i].Accrual, orders[i].UploadedAt, orders[i].Number); err != nil {
			log.Println(err.Error()) //	если при вставке произошла ошибка, то заносим её в журнал
		}
	}

	return tx.Commit() //	фиксируем транзакцию, и результат фиксации возвращаем в вызывающую функцию
}

//	syncStatusWithBonusServer - метод синхронизации списка заказов с сервером начисления бонусных баллов
func syncStatusWithBonusServer(orders []Order, AccrualAddress string) error {

	//	ЭТО ЗАГЛУШКА ДЛЯ ТЕСТОВЫХ НУЖД
	if AccrualAddress == "" { //	если сервер начисления баллов не задан, то включаем тестовый режим,
		//	в этом режиме все заказы принимаются безусловно, с переводом их в статус PROCESSED, с начислением 100 баллов
		//	дату загрузки заказа установим в "2022-01-01T00:00:00+03:00" - просто для определенности в тестах
		for i := range orders {
			orders[i].Status = "PROCESSED"
			orders[i].Accrual = 100
			orders[i].UploadedAt = "2022-01-01T00:00:00+03:00"
		}
		return nil //	завершаем процесс синхронизации
	} //	ВОТ И ВСЯ ЗАГЛУШКА

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

			errParsing := json.Unmarshal(body, &ordersUpdated) //	парсим JSON и записываем результат в ordersUpdated

			if errParsing != nil { //				проверяем успешно ли парсится JSON
				log.Println(errParsing.Error()) //	запишем в лог сообщение об ошибке
				continue                        //	и продолжаем цикл в новой итерации
			}
			//	нас интересуют только заказы перешедшие в финальные статусы - PROCESSED и INVALID
			//	меняем в списке orders для них статус и сумму начислений - на актуальные значения
			if ordersUpdated.Status == "PROCESSED" || ordersUpdated.Status == "INVALID" {
				orders[i].Status = ordersUpdated.Status
				orders[i].Accrual = ordersUpdated.Accrual
			}
		}
	}
	return nil
}
