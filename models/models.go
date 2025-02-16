package models

import "time"

type User struct {
	ID       int
	Username string
	Password string
	Coins    int
}

type Transaction struct {
	ID         int
	FromUserID int
	ToUserID   int
	Amount     int
	CreatedAt  time.Time
}

type Purchase struct {
	ID        int
	UserID    int
	Item      string
	Quantity  int
	CreatedAt time.Time
}
