package livekit

import (
	"errors"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/livekit/protocol/auth"
)

type LiveKitConfig struct {
	ApiKey    string
	ApiUrl    string
	ApiSecret string
}

var (
	ErrNoDataInEnv = errors.New("error. Failed to find data in .env")
)

func NewLiveKitConfig() (*LiveKitConfig, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}
	url := os.Getenv("LIVEKIT_URL")
	secret := os.Getenv("LIVEKIT_API_SECRET")
	apiKey := os.Getenv("LIVEKIT_API_KEY")

	if url == "" || secret == "" || apiKey == "" {
		return nil, ErrNoDataInEnv
	}
	return &LiveKitConfig{
		ApiKey:    apiKey,
		ApiSecret: secret,
		ApiUrl:    url,
	}, nil
}

func (c *LiveKitConfig) GenerateToken(room string, id string) (string, error) {
	at := auth.NewAccessToken(c.ApiKey, c.ApiSecret)

	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	at.SetVideoGrant(grant).SetIdentity(id).SetValidFor(time.Hour * 8)
	return at.ToJWT()
}
