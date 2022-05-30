package storage

import (
	"crypto/rand"
	"errors"
	"fmt"
)

//	Datasource - интерфейс источника данных сервера
//	может реализовываться базой данных PostgreSQL (Database) или в тестовых целях - базой данных sqllite (SQLliteDB) в режиме "in memory"
type Datasource interface {
	UserRegister(userID, password string) (token string, err error)        //	регистрация пользователя
	UserAuthorise(userID, password string) (token string, err error)       //	авторизация пользователя
	GetOrders(userID string) ([]Order, error)                              //	запрос списка заказов пользователя
	GetBalance(userID string) (accrualSum, withdrawSum float32, err error) //	запрос баланса пользователя
	GetWithdrawals(userID string) ([]Withdraw, error)                      //	запрос на списание баллов пользователя
	OrderInsert(order string, userID string) error                         //	запрос от пользователя на регистрацию нового заказа
	WithdrawRequest(order string, sum float32, userID string) error        //	запрос пользователя на списание баллов
	Close()                                                                //	закрытие источника данных
	UpdateOrdersStatus(AccrualAddress string) error                        //	синхронизация статуса заказов с внешним сервисом начисления баллов
}

// newSessionID - функция генерирует идентификатор для сессии пользователя
func newSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	sessionID := fmt.Sprintf("%X", b)
	return sessionID
}

//	Order - структура для передачи информации о начисленных баллах за покупки
//	используется в методе GetOrders
type Order struct {
	Number     string  `json:"number"`      //  номер заказа, за который начисляем баллы
	Accrual    float32 `json:"accrual"`     //  рассчитанные баллы к начислению
	Status     string  `json:"status"`      //  статус расчёта начисления
	UploadedAt string  `json:"uploaded_at"` //  дата загрузки заказа в систему
}

//	Withdraw - структура для передачи информации о списании баллов в счёт покупки
//	используется в методе GerWithdrawals
type Withdraw struct {
	Order       string  `json:"order"`        //  номер заказа, в счёт которого списываются баллы
	Sum         float32 `json:"sum"`          //  сумма баллов к списанию в счёт оплаты заказа
	ProcessedAt string  `json:"processed_at"` //  дата вывода средств на оплату заказа баллами
}

//	ErrEmptyNotAllowed - ошибка возникающая при попытке вставить пустое значение в любое поле структуры хранения
var ErrEmptyNotAllowed = errors.New("empty value is not allowed")

//	ErrNoDataToAnswer - ошибка возникающая при попытке авторизоваться с неправильным логин и/или пароль
var ErrNoDataToAnswer = errors.New("there is no data to answer")

//	ErrOrderExistToAccount - ошибка возникающая при попытке вставить заказ, который уже привязан к аккаунту этого пользователя
var ErrOrderExistToAccount = errors.New("this order is already bind with your account")

//	ErrOrderExistToUser - ошибка возникающая при попытке вставить заказ, который уже привязан к аккаунту другого пользователя
var ErrOrderExistToAnother = errors.New("this order is already bind with another account")

//	ErrInsufficientFundsToAccount - ошибка возникающая при попытке списать сумму баллов, большую чем осталось на счёте
var ErrInsufficientFundsToAccount = errors.New("there are insufficient funds in the account")

//	ErrUserAlreadyExist - ошибка возникающая при попытке создать новый аккаунт с логином, уже существующим в нашей базе
var ErrUserAlreadyExist = errors.New("account with same login already exist")

//	ErrLoginPasswordIsWrong - ошибка возникающая при попытке авторизоваться с неправильным логин и/или пароль
var ErrLoginPasswordIsWrong = errors.New("login or password is incorrect")
