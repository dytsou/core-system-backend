package oauthprovider

import (
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleConfig struct {
	config *oauth2.Config
}

type GoogleOauth struct {
	ClientID     string `yaml:"client_id"     envconfig:"GOOGLE_OAUTH_CLIENT_ID"`
	ClientSecret string `yaml:"client_secret" envconfig:"GOOGLE_OAUTH_CLIENT_SECRET"`
}

func NewGoogleConfig(clientID, clientSecret, redirectURL string) *GoogleConfig {
	return &GoogleConfig{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

func (g *GoogleConfig) Name() string {
	return "google"
}

func (g *GoogleConfig) Config() *oauth2.Config {
	return g.config
}

func (g *GoogleConfig) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return g.config.Exchange(ctx, code)
}

func (g *GoogleConfig) GetUsername(email string) string {
	return strings.Split(email, "@")[0]
}

// GetUserInfo fetches user information from Google's userinfo API
func (g *GoogleConfig) GetUserInfo(ctx context.Context, token *oauth2.Token) (user.User, error) {
	client := g.config.Client(ctx, token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return user.User{}, fmt.Errorf("failed to get user info from Google: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return user.User{}, fmt.Errorf("oauth provider userinfo API returned status: %d", resp.StatusCode)
	}

	var googleUser struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return user.User{}, fmt.Errorf("failed to decode Google user info: %w", err)
	}

	return user.User{
		ID:        uuid.New(),
		Name:      googleUser.Name,
		Username:  g.GetUsername(googleUser.Email),
		AvatarUrl: googleUser.Picture,
		Role:      "user",
	}, nil
}
