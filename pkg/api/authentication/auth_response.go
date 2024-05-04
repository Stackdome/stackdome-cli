package authentication

import "time"

type AuthResponse struct {
	Username       string    `json:"username"`
	Team           string    `json:"team"`
	Namespace      string    `json:"namespace"`
	TokenValidTill time.Time `json:"tokenValidTill"`
}
