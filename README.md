# Avt Shop

## Описание

Сервис позволяет сотрудникам покупать мерч за монеты и передавать монеты другим пользователям.

Сервис предоставляет следующие возможности:
- Аутентификация (JWT). При первой аутентификации пользователь создаётся автоматически с 1000 монетами.
- Получение информации о кошельке, включая:
  - Баланс (количество монет)
  - Инвентарь (купленные товары)
  - Историю транзакций (переводы монет: кто отправил/получил и в каком количестве)
- Перевод монет между сотрудниками. Баланс не может стать отрицательным.
- Покупка мерча по фиксированным ценам (например, t-shirt – 80, cup – 20, book – 50, pen – 10, powerbank – 200, hoody – 300, umbrella – 200, socks – 10, wallet – 50, pink-hoody – 500).

## Стек технологий

- **Язык:** Go
- **База данных:** PostgreSQL (используется в Docker Compose)
- **Авторизация:** JWT
- **Тестирование:** Unit-тесты (с использованием gomock и testify) и интеграционные (E2E) тесты (с использованием httptest)
- **Линтер:** golangci-lint

## Запуск

### Локально через Docker Compose

1. Убедитесь, что у вас установлен Docker и Docker Compose.
2. Клонируйте репозиторий и перейдите в его корневую директорию.
3. Выполните команду:
   ```bash
   docker-compose up --build
   ```
4. Сервис будет доступен по адресу [http://localhost:8080](http://localhost:8080).

## Тестирование

### Запуск тестов

Для запуска всех тестов (unit и E2E):
```bash
go test -v ./...
```

### Проверка покрытия тестами

Для получения отчёта по покрытию:
```bash
go test -coverprofile="coverage.out" "./..."
go tool cover -html="coverage.out"
```

## Линтер

Для проверки качества кода используется [golangci-lint](https://github.com/golangci/golangci-lint).

### Файл конфигурации `.golangci.yaml`:

```yaml
run:
   deadline: 2m

issues:
   exclude-dirs:
      - vendor
   exclude-files:
      - ".*\\.gen\\.go"

linters:
   enable:
      - govet
      - errcheck
      - staticcheck
      - gosimple
      - unused
      - gocritic

linters-settings:
   shadow:
      check-shadowing: true
   errcheck:
      check-type-assertions: true

output:
   formats: colored-line-number
```

### Установка и запуск golangci-lint

1. Установите golangci-lint:
   ```bash
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```
2. Убедитесь, что каталог `$GOPATH/bin` (на Windows, например, `C:\Users\User\go\bin`) добавлен в PATH.
3. Запустите линтер:
   ```bash
   golangci-lint run
   ```

## Проверка работы API

Ниже приведён полный набор однострочных команд curl для проверки работы сервиса.

#### **1. Аутентификация**

**Testuser3:**
```cmd
curl -X POST "http://localhost:8080/api/auth" -H "Content-Type: application/json" -d "{\"username\": \"testuser3\", \"password\": \"testpass\"}"
```

**Testuser4:**
```cmd
curl -X POST "http://localhost:8080/api/auth" -H "Content-Type: application/json" -d "{\"username\": \"testuser4\", \"password\": \"testpass\"}"
```

#### **2. Получение информации о кошельке, инвентаре и истории транзакций**

**Testuser3:**
```cmd
curl -X GET "http://localhost:8080/api/info" -H "Authorization: Bearer Полученный токен"
```

**Testuser4:**
```cmd
curl -X GET "http://localhost:8080/api/info" -H "Authorization: Bearer Полученный токен"
```

#### **3. Перевод монет от testuser3 к testuser4**

Запрос выполняется от имени testuser3 (перевод 50 монет):
```cmd
curl -X POST "http://localhost:8080/api/sendCoin" -H "Content-Type: application/json" -H "Authorization: Bearer Полученный токен" -d "{\"toUser\": \"testuser4\", \"amount\": 50}"
```

#### **4. Покупка мерча (например, "t-shirt") от testuser3**

Запрос выполняется от имени testuser3 (стоимость t-shirt – 80 монет):
```cmd
curl -X GET "http://localhost:8080/api/buy/t-shirt" -H "Authorization: Bearer Полученный токен"
```

После выполнения этой команды баланс testuser3 уменьшится на 80 монет, а в его инвентаре появится запись о покупке "t-shirt".