package authentication

import "time"

type AuthResponse struct {
	Username       string    `json:"username"`
	Organisation   string    `json:"organisation"`
	TokenValidTill time.Time `json:"tokenValidTill"`
}
