package service

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"AVTproject/models"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

//go:generate mockgen -destination=./mocks/mock_repository.go -package=mocks AVTproject/service Repository

type Repository interface {
	GetUserByUsername(ctx context.Context, username string) (models.User, error)
	GetUserByID(ctx context.Context, id int) (models.User, error)
	CreateUser(ctx context.Context, username, password string) (int, error)
	UpdateUserCoins(ctx context.Context, id, delta int) error
	AddTransaction(ctx context.Context, fromUserID, toUserID, amount int) error
	GetUserTransactions(ctx context.Context, userID int) ([]models.Transaction, []models.Transaction, error)
	GetUserPurchases(ctx context.Context, userID int) ([]models.Purchase, error)
	AddPurchase(ctx context.Context, userID int, item string) error
}

type Service struct {
	repo      Repository
	jwtSecret string
}

func NewService(repo Repository, jwtSecret string) Service {
	return Service{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

type InfoResponse struct {
	Coins       int                 `json:"coins"`
	Inventory   []InventoryItem     `json:"inventory"`
	CoinHistory CoinHistoryResponse `json:"coinHistory"`
}

type InventoryItem struct {
	Type     string `json:"type"`
	Quantity int    `json:"quantity"`
}

type CoinHistoryResponse struct {
	Received []TransactionInfo `json:"received"`
	Sent     []TransactionInfo `json:"sent"`
}

type TransactionInfo struct {
	OtherUser string `json:"otherUser"`
	Amount    int    `json:"amount"`
}

func (s Service) Authenticate(
	ctx context.Context,
	username, password string,
) (string, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			hashed, err := bcryptHash(password)
			if err != nil {
				return "", err
			}
			userID, err := s.repo.CreateUser(ctx, username, hashed)
			if err != nil {
				return "", err
			}
			user = models.User{
				ID:       userID,
				Username: username,
				Password: hashed,
				Coins:    1000,
			}
		} else {
			return "", err
		}
	} else {
		if !bcryptCompare(user.Password, password) {
			return "", errors.New("неверные учетные данные")
		}
	}

	token, err := generateJWT(user, s.jwtSecret)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s Service) GetInfo(
	ctx context.Context,
	userID int,
) (InfoResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return InfoResponse{}, err
	}
	purchases, err := s.repo.GetUserPurchases(ctx, userID)
	if err != nil {
		return InfoResponse{}, err
	}
	rec, sent, err := s.repo.GetUserTransactions(ctx, userID)
	if err != nil {
		return InfoResponse{}, err
	}

	var inventory []InventoryItem
	for _, p := range purchases {
		inventory = append(
			inventory,
			InventoryItem{
				Type:     p.Item,
				Quantity: p.Quantity,
			},
		)
	}

	var received []TransactionInfo
	for _, t := range rec {
		received = append(
			received,
			TransactionInfo{
				OtherUser: itoa(t.FromUserID),
				Amount:    t.Amount,
			},
		)
	}

	var sentInfo []TransactionInfo
	for _, t := range sent {
		sentInfo = append(
			sentInfo,
			TransactionInfo{
				OtherUser: itoa(t.ToUserID),
				Amount:    t.Amount,
			},
		)
	}

	return InfoResponse{
		Coins:       user.Coins,
		Inventory:   inventory,
		CoinHistory: CoinHistoryResponse{Received: received, Sent: sentInfo},
	}, nil
}

func (s Service) SendCoin(
	ctx context.Context,
	fromUserID int,
	toUsername string,
	amount int,
) error {
	receiver, err := s.repo.GetUserByUsername(ctx, toUsername)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateUserCoins(ctx, fromUserID, -amount); err != nil {
		return err
	}
	if err := s.repo.UpdateUserCoins(ctx, receiver.ID, amount); err != nil {
		return err
	}
	return s.repo.AddTransaction(ctx, fromUserID, receiver.ID, amount)
}

func (s Service) BuyItem(
	ctx context.Context,
	userID int,
	item string,
) error {
	merchPrices := map[string]int{
		"t-shirt":    80,
		"cup":        20,
		"book":       50,
		"pen":        10,
		"powerbank":  200,
		"hoody":      300,
		"umbrella":   200,
		"socks":      10,
		"wallet":     50,
		"pink-hoody": 500,
	}
	price, ok := merchPrices[item]
	if !ok {
		return errors.New("неверное название мерча")
	}
	if err := s.repo.UpdateUserCoins(ctx, userID, -price); err != nil {
		return err
	}
	return s.repo.AddPurchase(ctx, userID, item)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func bcryptCompare(hashed, password string) bool {
	err := bcrypt.CompareHashAndPassword(
		[]byte(hashed),
		[]byte(password),
	)
	return err == nil
}

func generateJWT(
	user models.User,
	secret string,
) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.MapClaims{
			"user_id":  user.ID,
			"username": user.Username,
			"exp":      time.Now().Add(24 * time.Hour).Unix(),
		},
	)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}
