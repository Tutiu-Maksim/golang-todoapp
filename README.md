# Golang Todo App

Приложение для ведения списка задач. Через веб-интерфейс и REST API можно создавать пользователей, добавлять задачи с указанием автора, отмечать их выполненными и смотреть статистику.

## Архитектура

- `cmd/todoapp` — точка входа. Собирает конфигурацию, пул подключений PostgreSQL, сервисы и HTTP-сервер.
- `internal/features/{users,tasks,statistics}` — модули API. Слой `transport/http` принимает запросы и формирует ответы, `service` выполняет операции приложения, `repository/postgres` работает с базой.
- `internal/core` — модели и валидация, обработка HTTP-ошибок, логирование, конфигурация и адаптер pgx.
- `public/index.html` — веб-интерфейс, который отдаёт модуль `internal/features/web`. Описание API находится в `docs`, миграции PostgreSQL — в `migrations`.

Пользователь является автором задачи. PostgreSQL хранит ссылку на автора и не позволяет удалить пользователя с существующими задачами. У пользователей и задач есть поле `version`. При обновлении оно увеличивается и защищает от одновременной перезаписи.

## Запуск в Docker

Нужны Docker с плагином Compose и Make. Выполните из корня проекта. Если `.env` уже есть, сохраните его и проверьте значения по `.env.example`.

```sh
cp .env.example .env
make todoapp-deploy
make ps
```

Compose дождётся готовности PostgreSQL, применит миграции и запустит приложение. По умолчанию доступны [интерфейс](http://localhost:5050/), [Swagger UI](http://localhost:5050/swagger/index.html) и API по адресу `http://localhost:5050/api/v1`.

`HTTP_PORT` задаёт внешний порт контейнера, `HTTP_ADDR` — адрес сервера при запуске через Go. `POSTGRES_USER`, `POSTGRES_PASSWORD` и `POSTGRES_DB` задают параметры базы. Для этих значений используйте символы, допустимые в URL без кодирования. `TIME_ZONE` задаёт часовой пояс приложения, например `Europe/Moscow`. Данные PostgreSQL хранятся в `out/pgdata`, логи приложения — в `out/logs`.

```sh
docker compose --profile tools down
```

Эта команда останавливает Compose и удаляет контейнеры. Данные и логи на диске сохраняются.

## Запуск Go-сервера на хосте

Понадобится Go 1.26. Подготовьте `.env` по примеру выше. Если контейнер приложения уже занимает HTTP-порт, остановите его командой `make todoapp-undeploy`.

```sh
make env-up
make migrate-up
make env-port-forward
make todoapp-run
```

Переадресация PostgreSQL доступна только на `127.0.0.1`. Если порт 5432 занят, измените `POSTGRES_PORT` в `.env`; Go-сервер подключится к тому же порту. После остановки Go-сервера команда `make env-down` остановит PostgreSQL и переадресацию.

| Команда | Действие |
| --- | --- |
| `make migrate-up` | Применить ожидающие миграции |
| `make migrate-create seq=add_field` | Создать пару SQL-файлов для миграции |
| `make migrate-down` | Откатить все миграции после подтверждения. Начальный откат удаляет таблицы и данные |
| `make env-port-close` | Остановить переадресацию PostgreSQL |
| `make swagger-gen` | Перегенерировать файлы Swagger в `docs` |
| `make logs-clear` | Удалить файлы логов после подтверждения |
| `make env-cleanup` | Остановить Compose и удалить данные PostgreSQL после подтверждения |

При обновлении существующей базы миграция `000002` приводит пустые описания задач к `NULL`. После обновления описание может быть `NULL` или содержать от 1 до 1000 символов. Откат возвращает прежнее ограничение базы, а преобразованные описания остаются `NULL`.

## Примеры API

Команды используют `curl` и `jq`. В примере идентификаторы берутся из ответов сервера.

```sh
API=http://localhost:5050/api/v1

USER_ID=$(curl -fsS -X POST "$API/users" \
  -H 'Content-Type: application/json' \
  -d '{"full_name":"Анна Смирнова","phone_number":"+79998887766"}' | jq -r '.id')

curl -fsS "$API/users/$USER_ID"
curl -fsS "$API/users?limit=10&offset=0"

curl -fsS -X PATCH "$API/users/$USER_ID" \
  -H 'Content-Type: application/json' \
  -d '{"phone_number":null}'

TASK_ID=$(curl -fsS -X POST "$API/tasks" \
  -H 'Content-Type: application/json' \
  -d "{\"title\":\"Проверить релиз\",\"author_user_id\":$USER_ID}" | jq -r '.id')

curl -fsS "$API/tasks/$TASK_ID"
curl -fsS "$API/tasks?user_id=$USER_ID&limit=10&offset=0"

curl -fsS -X PATCH "$API/tasks/$TASK_ID" \
  -H 'Content-Type: application/json' \
  -d '{"completed":true}'

curl -fsS "$API/statistics?user_id=$USER_ID"

curl -i -X DELETE "$API/tasks/$TASK_ID"
curl -i "$API/tasks/$TASK_ID"
curl -i -X DELETE "$API/users/$USER_ID"
```

`POST` возвращает `201 Created`, успешные `GET` и `PATCH` — `200 OK`, `DELETE` — `204 No Content`. Запрос удалённой задачи вернёт `404 Not Found`, а неверный идентификатор, например `/tasks/abc`, — `400 Bad Request`. При конкурентном обновлении возможен `409 Conflict`. Тело ошибки содержит поля `error` и `message`.

У `GET /statistics` есть необязательные параметры `user_id`, `from` и `to`. Даты передаются в формате `YYYY-MM-DD`.

## Тесты

По умолчанию `make test` запускает тесты без подключения к базе. Для проверки CRUD, статистики, ошибок API и миграции запустите PostgreSQL с доступом по TCP. Тест создаёт и удаляет собственную временную базу. Роли из URL нужно право `CREATEDB`.

```sh
make env-port-forward
TODOAPP_TEST_DATABASE_URL='postgres://todoapp:todoapp_dev@127.0.0.1:5432/todoapp?sslmode=disable' make test
```

Если вы изменили порт или учётные данные в `.env`, подставьте их в URL.

`make test-cover` измеряет покрытие операторов в пакетах `internal/...` и записывает профиль в `out/coverage.out`. Отчёт `out/coverage.html` показывает покрытые строки по файлам. Чтобы учесть интеграционные сценарии, запустите `make test-cover` с той же переменной `TODOAPP_TEST_DATABASE_URL`.
