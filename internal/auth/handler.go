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
	GenerateRefreshToken(ctx context.Context, user user.User) (jwt.RefreshToken, error)
	GetUserIDByRefreshToken(ctx context.Context, refreshTokenID uuid.UUID) (string, error)
}

type JWTStore interface {
	InactivateRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error
	InactivateRefreshToken(ctx context.Context, refreshTokenID uuid.UUID) error
}

type UserStore interface {
	Create(ctx context.Context, name, username, avatarUrl, role string) (user.User, error)
	ExistsByID(ctx context.Context, id uuid.UUID) (bool, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (user.User, error)
	FindOrCreate(ctx context.Context, name, username, avatarUrl, role string) (user.User, error)
}

type OAuthProvider interface {
	Name() string
	Config() *oauth2.Config
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (user.User, error)
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
				fmt.Sprintf("%s/api/oauth/google/callback", config.BaseURL),
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
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)

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
		http.Redirect(w, r, fmt.Sprintf("%s?error=%s", callback, oauthError), http.StatusTemporaryRedirect)
		return
	}

	token, err := provider.Exchange(traceCtx, code)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("%w: %v", internal.ErrInvalidExchangeToken, err), logger)
		return
	}

	userInfo, err := provider.GetUserInfo(traceCtx, token)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	jwtUser, err := h.userStore.FindOrCreate(traceCtx, userInfo.Name, userInfo.Username, userInfo.AvatarUrl, userInfo.Role)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	jwtToken, refreshTokenID, err := h.generateJWT(traceCtx, jwtUser)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	var redirectWithToken string
	if redirectTo != "" {
		redirectWithToken = fmt.Sprintf("%s?token=%s&refreshToken=%s&r=%s", callback, jwtToken, refreshTokenID, redirectTo)
	} else {
		redirectWithToken = fmt.Sprintf("%s?token=%s&refreshToken=%s", callback, jwtToken, refreshTokenID)
	}

	http.Redirect(w, r, redirectWithToken, http.StatusTemporaryRedirect)
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

func (h *Handler) generateJWT(ctx context.Context, user user.User) (string, string, error) {
	traceCtx, span := h.tracer.Start(ctx, "generateJWT")
	defer span.End()

	jwtToken, err := h.jwtIssuer.New(traceCtx, user)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := h.jwtIssuer.GenerateRefreshToken(traceCtx, user)
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

	user, err := h.jwtIssuer.Parse(traceCtx, r.Header.Get("Authorization"))
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	err = h.jwtStore.InactivateRefreshTokensByUserID(traceCtx, user.ID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, err, logger)
		return
	}

	// Clear the refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "accessToken",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !h.config.Debug,
		SameSite: http.SameSiteStrictMode,
	})

	handlerutil.WriteJSONResponse(w, http.StatusOK, map[string]string{"message": "Successfully logged out"})
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	traceCtx, span := h.tracer.Start(r.Context(), "RefreshToken")
	defer span.End()
	logger := logutil.WithContext(traceCtx, h.logger)

	refreshTokenStr := r.PathValue("refreshToken")
	if refreshTokenStr == "" {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("refresh token is required"), logger)
		return
	}

	refreshTokenID, err := uuid.Parse(refreshTokenStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid refresh token format"), logger)
		return
	}

	userIDStr, err := h.jwtIssuer.GetUserIDByRefreshToken(traceCtx, refreshTokenID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid or expired refresh token"), logger)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("invalid user ID"), logger)
		return
	}

	user, err := h.userStore.GetUserByID(traceCtx, userID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("user not found"), logger)
		return
	}

	err = h.jwtStore.InactivateRefreshToken(traceCtx, refreshTokenID)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to invalidate refresh token"), logger)
		return
	}

	newJWTToken, err := h.jwtIssuer.New(traceCtx, user)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to generate new JWT token"), logger)
		return
	}

	newRefreshToken, err := h.jwtIssuer.GenerateRefreshToken(traceCtx, user)
	if err != nil {
		h.problemWriter.WriteError(traceCtx, w, fmt.Errorf("failed to generate new refresh token"), logger)
		return
	}

	// Return new tokens according to project specification
	response := map[string]interface{}{
		"accessToken":    newJWTToken,
		"expirationTime": newRefreshToken.ExpirationDate.Time.UnixMilli(),
		"refreshToken":   newRefreshToken.ID.String(),
	}

	handlerutil.WriteJSONResponse(w, http.StatusOK, response)
}
