package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"AVTproject/models"
	"AVTproject/service"

	"AVTproject/service/mocks"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestService_Authenticate(t *testing.T) {
	type fields struct {
		prepareRepository func(*mocks.MockRepository)
	}
	type args struct {
		username string
		password string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantErr    bool
		wantUserID int
	}{
		{
			name: "New user creation",
			fields: fields{
				prepareRepository: func(mr *mocks.MockRepository) {
					mr.EXPECT().
						GetUserByUsername(gomock.Any(), "newuser").
						Return(models.User{}, sql.ErrNoRows)
					mr.EXPECT().
						CreateUser(gomock.Any(), "newuser", gomock.Any()).
						Return(1, nil)
				},
			},
			args: args{
				username: "newuser",
				password: "pass",
			},
			wantErr:    false,
			wantUserID: 1,
		},
		{
			name: "Existing user, correct password",
			fields: fields{
				prepareRepository: func(mr *mocks.MockRepository) {
					pass := "pass"
					hashed, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
					user := models.User{
						ID:       2,
						Username: "existing",
						Password: string(hashed),
						Coins:    1000,
					}
					mr.EXPECT().
						GetUserByUsername(gomock.Any(), "existing").
						Return(user, nil)
				},
			},
			args: args{
				username: "existing",
				password: "pass",
			},
			wantErr:    false,
			wantUserID: 2,
		},
		{
			name: "Existing user, wrong password",
			fields: fields{
				prepareRepository: func(mr *mocks.MockRepository) {
					pass := "pass"
					hashed, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
					user := models.User{
						ID:       3,
						Username: "existing",
						Password: string(hashed),
						Coins:    1000,
					}
					mr.EXPECT().
						GetUserByUsername(gomock.Any(), "existing").
						Return(user, nil)
				},
			},
			args: args{
				username: "existing",
				password: "wrongpass",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockRepo := mocks.NewMockRepository(ctrl)
			tt.fields.prepareRepository(mockRepo)

			svc := service.NewService(mockRepo, "secret")
			token, err := svc.Authenticate(ctx, tt.args.username, tt.args.password)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, token)

			parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
				return []byte("secret"), nil
			})
			require.NoError(t, err)
			claims, ok := parsed.Claims.(jwt.MapClaims)
			require.True(t, ok)
			userID := int(claims["user_id"].(float64))
			require.Equal(t, tt.wantUserID, userID)
			require.Equal(t, tt.args.username, claims["username"])
		})
	}
}

func TestService_BuyItem_InsufficientCoins(t *testing.T) {
	type fields struct {
		prepareRepository func(*mocks.MockRepository)
	}
	type args struct {
		userID int
		item   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Insufficient coins for t-shirt",
			fields: fields{
				prepareRepository: func(mr *mocks.MockRepository) {
					mr.EXPECT().
						UpdateUserCoins(gomock.Any(), 3, -80).
						Return(errors.New("недостаточно монет"))
				},
			},
			args: args{
				userID: 3,
				item:   "t-shirt",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockRepo := mocks.NewMockRepository(ctrl)
			tt.fields.prepareRepository(mockRepo)

			svc := service.NewService(mockRepo, "secret")
			err := svc.BuyItem(ctx, tt.args.userID, tt.args.item)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_SendCoin_Success(t *testing.T) {
	type fields struct {
		prepareRepository func(*mocks.MockRepository)
	}
	type args struct {
		senderID   int
		toUsername string
		amount     int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Successful coin transfer",
			fields: fields{
				prepareRepository: func(mr *mocks.MockRepository) {
					receiver := models.User{ID: 4, Username: "testuser4", Coins: 1000}
					mr.EXPECT().
						GetUserByUsername(gomock.Any(), "testuser4").
						Return(receiver, nil)
					mr.EXPECT().
						UpdateUserCoins(gomock.Any(), 3, -50).
						Return(nil)
					mr.EXPECT().
						UpdateUserCoins(gomock.Any(), 4, 50).
						Return(nil)
					mr.EXPECT().
						AddTransaction(gomock.Any(), 3, 4, 50).
						Return(nil)
				},
			},
			args: args{
				senderID:   3,
				toUsername: "testuser4",
				amount:     50,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			mockRepo := mocks.NewMockRepository(ctrl)
			tt.fields.prepareRepository(mockRepo)

			svc := service.NewService(mockRepo, "secret")
			err := svc.SendCoin(ctx, tt.args.senderID, tt.args.toUsername, tt.args.amount)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
