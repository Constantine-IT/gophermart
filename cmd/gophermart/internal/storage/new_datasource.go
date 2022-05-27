package storage

import (
	"database/sql"
	//	github.com/jackc/pgx/stdlib - драйвер PostgreSQL для доступа к БД с использованием пакета database/sql
	//	если хотим работать с БД напрямую, без database/sql надо использовать пакет - github.com/jackc/pgx/v4
	_ "github.com/jackc/pgx/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

// NewDatasource - функция конструктор, инициализирующая хранилище
func NewDatasource(DatabaseDSN string) (strg Datasource, err error) {

	var d Database

	//	если не задана переменная среды DATABASE_DSN, то работаем с БД - sqllite3
	if DatabaseDSN == "" { //	режим - "in memory" - всё в оперативке, на диске файлов НЕ создается
		d.DB, err = sql.Open("sqlite3", ":memory:") //	при перезагрузке всё содержимое БД теряется
		if err != nil {
			return nil, err
		}
	} else { //	если задана переменная среды DATABASE_DSN, то работаем с БД - Postgres
		//	открываем connect с базой данных PostgreSQL 10+
		d.DB, err = sql.Open("pgx", DatabaseDSN)
		if err != nil { //	при ошибке открытия, прерываем работу конструктора
			return nil, err
		}
		//	тестируем доступность базы данных
		if err := d.DB.Ping(); err != nil { //	если база недоступна, прерываем работу конструктора
			return nil, err
		}
	}

	//	если база данных доступна - создаём в ней структуры хранения
	//	готовим SQL-statement для создания таблицы со списком пользователей, если её не существует
	stmt := `create table if not exists "users" (
						"userid" TEXT constraint userid_pk primary key not null,
						"password" TEXT not null,
                        "session_id" TEXT constraint session_id_uniq unique not null)`
	_, err = d.DB.Exec(stmt)
	if err != nil { //	при ошибке в создании структур хранения в базе данных, прерываем работу конструктора
		return nil, err
	}

	//	готовим SQL-statement для создания таблицы заказов для начисления баллов, если её не существует
	stmt = `create table if not exists "orders" (
						"order" TEXT constraint orders_pk primary key not null,
						"status" TEXT not null,
   					"accrual" NUMERIC not null,
   					"uploaded_at" TEXT not null,
						"userid" TEXT not null)`
	_, err = d.DB.Exec(stmt)
	if err != nil { //	при ошибке в создании структур хранения в базе данных, прерываем работу конструктора
		return nil, err
	}

	//	готовим SQL-statement для создания таблицы списаний баллов, если её не существует
	stmt = `create table if not exists "withdrawals" (
					"order" TEXT constraint withdrawals_pk primary key not null,
					"sum" NUMERIC not null,
					"processed_at" TEXT not null,
					"userid" TEXT not null)`

	_, err = d.DB.Exec(stmt)
	if err != nil { //	при ошибке в создании структур хранения в базе данных, прерываем работу конструктора
		return nil, err
	}

	strg = &Database{DB: d.DB}

	return strg, nil //	если всё прошло ОК, то возвращаем выбранный источник данных
}
