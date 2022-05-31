package storage

import (
	"log"
	"net/http"
	"time"

	"encoding/json"
	"github.com/go-resty/resty/v2"
)

//	BonusServer - сервер начисления бонусных баллов
type BonusServer struct {
	AccrualAddress string //	адрес сервера
}

//	SyncOrderStatus - метод синхронизации списка заказов с сервером начисления бонусных баллов
func (s *BonusServer) SyncOrderStatus(orders []Order) error {
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
		resp, err := client.R().Get(s.AccrualAddress + "/api/orders/" + orders[i].Number)
		if err != nil {
			return err
		}

		status := resp.StatusCode() //	считываем код статуса ответа

		for status == http.StatusTooManyRequests { //	если пришел ответ со статусом 429 - TooManyRequests
			log.Println("response status - TooManyRequests for sync service")
			time.Sleep(5 * time.Second) //	если превышен лимит количества запросов в минуту, делаем паузу
			//	и повторяем запрос с теми же параметрами
			resp, err := client.R().Get(s.AccrualAddress + "/api/orders/" + orders[i].Number)
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
	return nil //	завершаем процесс синхронизации
}

//	MOKServer - эмулятор сервера начисления бонусных баллов для тестов
type MOKServer struct{}

//	SyncOrderStatus - метод синхронизации списка заказов с сервером начисления бонусных баллов
func (s *MOKServer) SyncOrderStatus(orders []Order) error {
	for i := range orders { //									в эмуляторе все заказы принимаются безусловно
		orders[i].Status = "PROCESSED"                     //	с переводом их в статус PROCESSED
		orders[i].Accrual = 100                            //	с начислением 100 баллов
		orders[i].UploadedAt = "2022-01-01T00:00:00+03:00" //	и датой загрузки заказа - "2022-01-01T00:00:00+03:00"
	}
	return nil //	завершаем процесс синхронизации
}
