package auth

import (
	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/auth/oauthprovider"
	"NYCU-SDC/core-system-backend/internal/config"
	"NYCU-SDC/core-system-backend/internal/jwt"
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	handlerutil "github.com/NYCU-SDC/summer/pkg/handler"
	logutil "github.com/NYCU-SDC/summer/pkg/log"
	"github.com/NYCU-SDC/summer/pkg/problem"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type JWTIssuer interface {
	New(ctx context.Context, user user.User) (string, error)
	Parse(ctx context.Context, tokenString string) (user.User, error)
	GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (jwt.RefreshToken, error)
	GetUserIDByRefreshToken(ctx context.Context, refreshTokenID uuid.UUID) (uuid.UUID, error)
}

type JWTStore interface {
	InactivateRefreshToken(ctx context.Context, id uuid.UUID) error
	GetRefreshTokenByID(ctx context.Context, id uuid.UUID) (jwt.RefreshToken, error)
}

type UserStore interface {
	GetUserIDByID(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (user.User, error)
	FindOrCreate(ctx context.Context, name, username, avatarUrl string, role []string, oauthProvider, oauthProviderID string) (uuid.UUID, error)
}

type OAuthProvider interface {
	Name() string
	Config() *oauth2.Config
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (user.User, user.Auth, error)
}

type callBackInfo struct {
	code       string
	oauthError string
	callback   url.URL
	redirectTo string
}

type Handler struct {
	config config.Config
	logger *zap.Logger
	tracer trace.Tracer

	validator     *validator.Validate
	problemWriter *problem.HttpWriter

	userStore UserStore
	jwtIssuer JWTIssuer
	jwtStore  JWTStore
	provider  map[string]OAuthProvider
}

func NewHandler(
	config config.Config,
	logger *zap.Logger,
	validator *validator.Validate,
	problemWriter *problem.HttpWriter,
	userStore UserStore,
	jwtIssuer JWTIssuer,
	jwtStore JWTStore,
) *Handler {

	return &Handler{
		config: config,
		logger: logger,
		tracer: otel.Tracer("auth/handler"),

		validator:     validator,
		problemWriter: problemWriter,

		userStore: userStore,
		jwtIssuer: jwtIssuer,
		jwtStore:  jwtStore,
		provider: map[string]OAuthProvider{
			"google": oauthprovider.NewGoogleConfig(
				config.GoogleOauth.ClientID,
				config.GoogleOauth.ClientSecret,
				fmt.Sprintf("%s/api/auth/login/oauth/google/callback", config.BaseURL),
			),
		},
	}
}

// Oauth2Start initiates the OAuth2 flow by redirecting the user to the provider's authorization URL
func (h *Handler) Oauth2Start(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "Oauth2Start")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	providerName := r.PathValue("provider")
	provider := h.provider[providerName]
	if provider == nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("%w: provider not found: %s", internal.ErrProviderNotFound, providerName), logger)
		return
	}

	callback := r.URL.Query().Get("c")
	redirectTo := r.URL.Query().Get("r")
	if callback == "" {
		callback = fmt.Sprintf("%s/api/oauth/debug/token", h.config.BaseURL)
	}
	if redirectTo != "" {
		callback = fmt.Sprintf("%s?r=%s", callback, redirectTo)
	}
	state := base64.StdEncoding.EncodeToString([]byte(callback))

	authURL := provider.Config().AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, authURL, http.StatusFound)

	logger.Info("Redirecting to Google OAuth2", zap.String("url", authURL))
}

// Callback handles the OAuth2 callback from the provider
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "Callback")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	providerName := r.PathValue("provider")
	provider := h.provider[providerName]
	if provider == nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("%w: provider not found: %s", internal.ErrProviderNotFound, providerName), logger)
		return
	}

	// Get the OAuth2 code and state from the request
	callbackInfo, err := h.getCallBackInfo(r.URL)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("%w: %v", internal.ErrInvalidCallbackInfo, err), logger)
		return
	}

	callback := callbackInfo.callback.String()
	code := callbackInfo.code
	redirectTo := callbackInfo.redirectTo
	oauthError := callbackInfo.oauthError

	if oauthError != "" {
		http.Redirect(w, r, fmt.Sprintf("%s?error=%s", callback, oauthError), http.StatusFound)
		return
	}

	token, err := provider.Exchange(traceCtx, code)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("%w: %v", internal.ErrInvalidExchangeToken, err), logger)
		return
	}

	userInfo, auth, err := provider.GetUserInfo(traceCtx, token)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	userID, err := h.userStore.FindOrCreate(traceCtx, userInfo.Name.String, userInfo.Username.String, userInfo.AvatarUrl.String, userInfo.Role, providerName, auth.ProviderID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	accessTokenID, refreshTokenID, err := h.generateJWT(traceCtx, userID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	// Set access token as HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessTokenID,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   15 * 60,
	})

	// Set refresh token as HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshTokenID,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/auth/refresh",
		MaxAge:   30 * 24 * 60 * 60,
	})

	var redirectURL string
	if redirectTo != "" {
		redirectURL = fmt.Sprintf("%s?r=%s", callback, redirectTo)
	} else {
		redirectURL = callback
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *Handler) DebugToken(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "DebugToken")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	e := r.URL.Query().Get("error")
	if e != "" {
		h.problemWriter.WriteError(traceCtx, w, handlerutil.ErrForbidden, logger)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		h.problemWriter.WriteError(traceCtx, w, errors.New("missing token"), logger)
		return
	}

	jwtUser, err := h.jwtIssuer.Parse(traceCtx, token)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, jwtUser)
}

func (h *Handler) generateJWT(ctx context.Context, userID uuid.UUID) (string, string, error) {
	traceCtx, span := h.tracer.Start(ctx, "generateJWT")
	defer span.End()

	user, err := h.userStore.GetUserByID(traceCtx, userID)
	if err != nil {
		return "", "", err
	}

	jwtToken, err := h.jwtIssuer.New(traceCtx, user)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := h.jwtIssuer.GenerateRefreshToken(traceCtx, userID)
	if err != nil {
		return "", "", err
	}

	return jwtToken, refreshToken.ID.String(), nil
}

func (h *Handler) getCallBackInfo(url *url.URL) (callBackInfo, error) {

	code := url.Query().Get("code")
	state := url.Query().Get("state")
	oauthError := url.Query().Get("error")

	callbackURL, err := base64.StdEncoding.DecodeString(state)
	if err != nil {
		return callBackInfo{}, err
	}

	callback, err := url.Parse(string(callbackURL))
	if err != nil {
		return callBackInfo{}, err
	}

	// Clear the query parameters from the callback URL, due to "?" symbol in original URL
	redirectTo := callback.Query().Get("r")
	callback.RawQuery = ""

	return callBackInfo{
		code:       code,
		oauthError: oauthError,
		callback:   *callback,
		redirectTo: redirectTo,
	}, nil
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "Logout")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	// Inactivate the current refresh token from cookie
	refreshTokenCookie, err := r.Cookie("refresh_token")
	if err == nil && refreshTokenCookie.Value != "" {
		refreshTokenID, err := uuid.Parse(refreshTokenCookie.Value)
		if err == nil {
			err = h.jwtStore.InactivateRefreshToken(traceCtx, refreshTokenID)
			if err != nil {
				logger.Warn("Failed to inactivate refresh token during logout", zap.Error(err))
			}
		}
	}

	// Clear the access token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// Clear the refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth/refresh",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	handlerutil.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "Successfully logged out"})
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "RefreshToken")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	// Read refresh token from cookie instead of path parameter
	refreshTokenCookie, err := r.Cookie("refresh_token")
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("refresh token cookie is required"), logger)
		return
	}
	refreshTokenStr := refreshTokenCookie.Value

	if refreshTokenStr == "" {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("refresh token is required"), logger)
		return
	}

	refreshTokenID, err := uuid.Parse(refreshTokenStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid refresh token format"), logger)
		return
	}

	userID, err := h.jwtIssuer.GetUserIDByRefreshToken(traceCtx, refreshTokenID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid or expired refresh token"), logger)
		return
	}

	err = h.jwtStore.InactivateRefreshToken(traceCtx, refreshTokenID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to invalidate current refresh token"), logger)
		return
	}

	newAccessTokenID, newRefreshTokenID, err := h.generateJWT(traceCtx, userID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	// Set new access token as HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    newAccessTokenID,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   15 * 60, // 15 minutes
	})

	// Set new refresh token as HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshTokenID,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/auth/refresh",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
	})

	newRefreshToken, err := h.jwtStore.GetRefreshTokenByID(traceCtx, uuid.MustParse(newRefreshTokenID))
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	response := map[string]interface{}{
		"accessToken":    newAccessTokenID,
		"expirationTime": newRefreshToken.ExpirationDate.Time.UnixMilli(),
		"refreshToken":   newRefreshToken.ID.String(),
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}
