package models

import "time"

type User struct {
	ID         string    `json:"id"`
	Username   string    `json:"username"`
	Password   string    `json:"-"`
	NGACNodeID string    `json:"ngac_node_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Company    string `json:"company"`
	Department string `json:"department"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  UserInfo `json:"user"`
}

type UserInfo struct {
	ID         string   `json:"id"`
	Username   string   `json:"username"`
	Company    string   `json:"company"`
	Department string   `json:"department"`
	NGACNodeID string   `json:"ngac_node_id"`
}
