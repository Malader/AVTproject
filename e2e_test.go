package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"AVTproject/handlers"
	"AVTproject/models"
	"AVTproject/service"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

type inMemRepository struct {
	mu             sync.Mutex
	users          map[int]models.User
	usersByName    map[string]models.User
	transactions   []models.Transaction
	purchases      []models.Purchase
	nextUserID     int
	nextTransID    int
	nextPurchaseID int
}

func newInMemRepository() *inMemRepository {
	return &inMemRepository{
		users:          make(map[int]models.User),
		usersByName:    make(map[string]models.User),
		transactions:   []models.Transaction{},
		purchases:      []models.Purchase{},
		nextUserID:     1,
		nextTransID:    1,
		nextPurchaseID: 1,
	}
}

func (r *inMemRepository) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.usersByName[username]
	if !ok {
		return models.User{}, sql.ErrNoRows
	}
	return user, nil
}

func (r *inMemRepository) GetUserByID(ctx context.Context, id int) (models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok {
		return models.User{}, sql.ErrNoRows
	}
	return user, nil
}

func (r *inMemRepository) CreateUser(ctx context.Context, username, password string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.nextUserID
	r.nextUserID++
	user := models.User{
		ID:       id,
		Username: username,
		Password: password,
		Coins:    1000,
	}
	r.users[id] = user
	r.usersByName[username] = user
	return id, nil
}

func (r *inMemRepository) UpdateUserCoins(ctx context.Context, id, delta int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok {
		return sql.ErrNoRows
	}
	newCoins := user.Coins + delta
	if newCoins < 0 {
		return errors.New("недостаточно монет")
	}
	user.Coins = newCoins
	r.users[id] = user
	r.usersByName[user.Username] = user
	return nil
}

func (r *inMemRepository) AddTransaction(ctx context.Context, fromUserID, toUserID, amount int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	trans := models.Transaction{
		ID:         r.nextTransID,
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Amount:     amount,
		CreatedAt:  time.Now(),
	}
	r.nextTransID++
	r.transactions = append(r.transactions, trans)
	return nil
}

func (r *inMemRepository) GetUserTransactions(ctx context.Context, userID int) ([]models.Transaction, []models.Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var received, sent []models.Transaction
	for _, t := range r.transactions {
		if t.ToUserID == userID {
			received = append(received, t)
		}
		if t.FromUserID == userID {
			sent = append(sent, t)
		}
	}
	return received, sent, nil
}

func (r *inMemRepository) GetUserPurchases(ctx context.Context, userID int) ([]models.Purchase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []models.Purchase
	for _, p := range r.purchases {
		if p.UserID == userID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (r *inMemRepository) AddPurchase(ctx context.Context, userID int, item string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, p := range r.purchases {
		if p.UserID == userID && p.Item == item {
			r.purchases[i].Quantity++
			return nil
		}
	}
	purchase := models.Purchase{
		ID:        r.nextPurchaseID,
		UserID:    userID,
		Item:      item,
		Quantity:  1,
		CreatedAt: time.Now(),
	}
	r.nextPurchaseID++
	r.purchases = append(r.purchases, purchase)
	return nil
}

func setupTestServer() *httptest.Server {
	repo := newInMemRepository()
	svc := service.NewService(repo, "secret")
	h := handlers.NewHandler(svc, "secret")

	r := mux.NewRouter()
	r.HandleFunc("/api/auth", h.AuthHandler).Methods("POST")
	r.HandleFunc("/api/info", h.JWTMiddleware(h.InfoHandler)).Methods("GET")
	r.HandleFunc("/api/sendCoin", h.JWTMiddleware(h.SendCoinHandler)).Methods("POST")
	r.HandleFunc("/api/buy/{item}", h.JWTMiddleware(h.BuyHandler)).Methods("GET")
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Test Server"))
	}).Methods("GET")
	return httptest.NewServer(r)
}

func TestE2E_BuyMerch(t *testing.T) {
	type args struct {
		username      string
		password      string
		item          string
		purchaseCount int
	}
	type expected struct {
		coins    int
		itemType string
		quantity int
	}
	tests := []struct {
		name     string
		args     args
		expected expected
	}{
		{
			name: "Успешная покупка t-shirt один раз: баланс 1000 -> 920",
			args: args{
				username:      "e2e_buyer",
				password:      "pass",
				item:          "t-shirt",
				purchaseCount: 1,
			},
			expected: expected{
				coins:    920,
				itemType: "t-shirt",
				quantity: 1,
			},
		},
		{
			name: "Покупка t-shirt два раза: баланс 1000 -> 840, 2 футболки в инвентаре",
			args: args{
				username:      "e2e_buyer2",
				password:      "pass",
				item:          "t-shirt",
				purchaseCount: 2,
			},
			expected: expected{
				coins:    840,
				itemType: "t-shirt",
				quantity: 2,
			},
		},
		{
			name: "Покупка мерча с недостаточным балансом: попытка купить 15 раз t-shirt (15*80=1200 > 1000)",
			args: args{
				username:      "e2e_buyer3",
				password:      "pass",
				item:          "t-shirt",
				purchaseCount: 15,
			},
			expected: expected{
				coins:    40,
				itemType: "t-shirt",
				quantity: 12,
			},
		},
	}

	ts := setupTestServer()
	defer ts.Close()
	client := ts.Client()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authPayload := map[string]string{
				"username": tt.args.username,
				"password": tt.args.password,
			}
			data, err := json.Marshal(authPayload)
			require.NoError(t, err)
			resp, err := client.Post(ts.URL+"/api/auth", "application/json", bytes.NewReader(data))
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var authResp map[string]string
			require.NoError(t, json.Unmarshal(body, &authResp))
			token := authResp["token"]
			require.NotEmpty(t, token)

			var purchaseErr error
			for i := 0; i < tt.args.purchaseCount; i++ {
				req, err := http.NewRequest("GET", ts.URL+"/api/buy/"+tt.args.item, nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
				resp, err := client.Do(req)
				require.NoError(t, err)
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					purchaseErr = errors.New("покупка не удалась")
					break
				}
			}

			if purchaseErr != nil {
				require.Error(t, purchaseErr, "Ожидалась ошибка при покупке товара, когда средств недостаточно")
				req, err := http.NewRequest("GET", ts.URL+"/api/info", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				var infoResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &infoResp))
				coins := int(infoResp["coins"].(float64))
				require.Equal(t, tt.expected.coins, coins)

				inv := infoResp["inventory"].([]interface{})
				if len(inv) > 0 {
					item := inv[0].(map[string]interface{})
					require.Equal(t, tt.expected.itemType, item["type"])
					qty := int(item["quantity"].(float64))
					require.Equal(t, tt.expected.quantity, qty)
				}
			} else {
				req, err := http.NewRequest("GET", ts.URL+"/api/info", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				var infoResp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &infoResp))
				coins := int(infoResp["coins"].(float64))
				require.Equal(t, tt.expected.coins, coins)

				inv := infoResp["inventory"].([]interface{})
				require.Equal(t, 1, len(inv))
				item := inv[0].(map[string]interface{})
				require.Equal(t, tt.expected.itemType, item["type"])
				qty := int(item["quantity"].(float64))
				require.Equal(t, tt.expected.quantity, qty)
			}

		})
	}
}

func TestE2E_SendCoin(t *testing.T) {
	type args struct {
		senderUsername   string
		senderPassword   string
		receiverUsername string
		receiverPassword string
		amount           int
	}
	type expected struct {
		senderCoins   int
		receiverCoins int
	}
	tests := []struct {
		name     string
		args     args
		expected expected
	}{
		{
			name: "Успешный перевод 100 монет от sender_e2e к receiver_e2e",
			args: args{
				senderUsername:   "e2e_sender",
				senderPassword:   "pass",
				receiverUsername: "e2e_receiver",
				receiverPassword: "pass",
				amount:           100,
			},
			expected: expected{
				senderCoins:   900,
				receiverCoins: 1100,
			},
		},
		{
			name: "Перевод с недостаточным балансом: после покупки мерча у отправителя",
			args: args{
				senderUsername:   "e2e_sender2",
				senderPassword:   "pass",
				receiverUsername: "e2e_receiver2",
				receiverPassword: "pass",
				amount:           950,
			},
			expected: expected{
				senderCoins:   920,
				receiverCoins: 1000,
			},
		},
		{
			name: "Перевод монет для нового получателя (автосоздание)",
			args: args{
				senderUsername:   "e2e_sender3",
				senderPassword:   "pass",
				receiverUsername: "new_receiver",
				receiverPassword: "pass",
				amount:           200,
			},
			expected: expected{
				senderCoins:   800,
				receiverCoins: 1200,
			},
		},
	}

	ts := setupTestServer()
	defer ts.Close()
	client := ts.Client()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			senderPayload := map[string]string{
				"username": tt.args.senderUsername,
				"password": tt.args.senderPassword,
			}
			data, err := json.Marshal(senderPayload)
			require.NoError(t, err)
			resp, err := client.Post(ts.URL+"/api/auth", "application/json", bytes.NewReader(data))
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var authResp map[string]string
			require.NoError(t, json.Unmarshal(body, &authResp))
			senderToken := authResp["token"]
			require.NotEmpty(t, senderToken)

			if tt.name == "Перевод с недостаточным балансом: после покупки мерча у отправителя" {
				req, err := http.NewRequest("GET", ts.URL+"/api/buy/t-shirt", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+senderToken)
				resp, err = client.Do(req)
				require.NoError(t, err)
				resp.Body.Close()
			}

			receiverPayload := map[string]string{
				"username": tt.args.receiverUsername,
				"password": tt.args.receiverPassword,
			}
			data, err = json.Marshal(receiverPayload)
			require.NoError(t, err)
			resp, err = client.Post(ts.URL+"/api/auth", "application/json", bytes.NewReader(data))
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &authResp))
			receiverToken := authResp["token"]
			require.NotEmpty(t, receiverToken)

			transferPayload := map[string]interface{}{
				"toUser": tt.args.receiverUsername,
				"amount": tt.args.amount,
			}
			data, err = json.Marshal(transferPayload)
			require.NoError(t, err)
			req, err := http.NewRequest("POST", ts.URL+"/api/sendCoin", bytes.NewReader(data))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+senderToken)
			resp, err = client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
			var transferResp map[string]string
			require.NoError(t, json.Unmarshal(body, &transferResp))

			if tt.expected.senderCoins == 920 {
				require.NotEqual(t, "ok", transferResp["status"])
			} else {
				require.Equal(t, "ok", transferResp["status"])
			}

			req, err = http.NewRequest("GET", ts.URL+"/api/info", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+senderToken)
			resp, err = client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
			var senderInfo map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &senderInfo))
			senderCoins := int(senderInfo["coins"].(float64))
			require.Equal(t, tt.expected.senderCoins, senderCoins)

			req, err = http.NewRequest("GET", ts.URL+"/api/info", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+receiverToken)
			resp, err = client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err = io.ReadAll(resp.Body)
			require.NoError(t, err)
			var receiverInfo map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &receiverInfo))
			receiverCoins := int(receiverInfo["coins"].(float64))
			require.Equal(t, tt.expected.receiverCoins, receiverCoins)
		})
	}
}
