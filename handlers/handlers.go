package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"AVTproject/service"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
)

type Handler struct {
	svc       service.Service
	jwtSecret string
}

func NewHandler(svc service.Service, jwtSecret string) Handler {
	return Handler{
		svc:       svc,
		jwtSecret: jwtSecret,
	}
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type SendCoinRequest struct {
	ToUser string `json:"toUser"`
	Amount int    `json:"amount"`
}

type ErrorResponse struct {
	Errors string `json:"errors"`
}

func (h Handler) AuthHandler(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Неверный запрос")
		return
	}
	token, err := h.svc.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, AuthResponse{Token: token})
}

func (h Handler) InfoHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Пользователь не найден в контексте")
		return
	}
	info, err := h.svc.GetInfo(r.Context(), userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, info)
}

func (h Handler) SendCoinHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Пользователь не найден в контексте")
		return
	}
	var req SendCoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Неверный запрос")
		return
	}
	if req.ToUser == "" || req.Amount <= 0 {
		respondWithError(w, http.StatusBadRequest, "Неверные параметры запроса")
		return
	}
	if err := h.svc.SendCoin(r.Context(), userID, req.ToUser, req.Amount); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h Handler) BuyHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Пользователь не найден в контексте")
		return
	}
	vars := mux.Vars(r)
	item, exists := vars["item"]
	if !exists {
		respondWithError(w, http.StatusBadRequest, "Название товара не указано")
		return
	}
	if err := h.svc.BuyItem(r.Context(), userID, item); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h Handler) JWTMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondWithError(w, http.StatusUnauthorized, "Отсутствует токен авторизации")
			return
		}

		const bearerPrefix = "Bearer "
		if len(authHeader) <= len(bearerPrefix) {
			respondWithError(w, http.StatusUnauthorized, "Неверный формат токена")
			return
		}

		tokenStr := authHeader[len(bearerPrefix):]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return []byte(h.jwtSecret), nil
		})
		if err != nil || !token.Valid {
			respondWithError(w, http.StatusUnauthorized, "Неверный токен")
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			uid, err := strconv.Atoi(stringify(claims["user_id"]))
			if err != nil {
				respondWithError(w, http.StatusUnauthorized, "Неверный идентификатор пользователя в токене")
				return
			}
			ctx := context.WithValue(r.Context(), "user_id", uid)
			next(w, r.WithContext(ctx))
		} else {
			respondWithError(w, http.StatusUnauthorized, "Неверные данные токена")
			return
		}
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Errors: message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func stringify(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return strconv.Itoa(int(v))
	default:
		return ""
	}
}
