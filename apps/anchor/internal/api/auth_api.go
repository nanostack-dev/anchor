package api

import (
	"context"
	"net/http"
	"time"

	"anchor/internal/domain/auth"
	"anchor/internal/domain/platform"
)

const (
	cookieLifetimeDays     = 30 // refresh token cookie lifetime in days
	refreshTokenCookieName = "refresh_token"
)

func mapAuthUserToAPIUserResponse(user *platform.User) UserResponse {
	return UserResponse{
		Id:    user.ID,
		Email: user.Email,
		Role:  user.Role.ToString(),
	}
}

func (s *AnchorAPI) setRefreshTokenCookie(token string, lifetimeSeconds int64) *string {
	isDev := s.CoreConfig.IsDevelopment()

	if isDev {
		s.logger.Warn().Msg("Running in development mode - using insecure cookie settings")
	}

	//nolint:gosec // Local development needs a non-Secure refresh cookie on localhost.
	cookie := http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    token,
		Path:     "/v1/auth/refresh",
		Expires:  time.Now().Add(time.Second * time.Duration(lifetimeSeconds)),
		HttpOnly: true,
		Secure:   !isDev, // false for development, true for production
		SameSite: func() http.SameSite {
			if isDev {
				return http.SameSiteLaxMode // Better for localhost development
			}
			return http.SameSiteNoneMode // Required for cross-origin in production
		}(),
	}
	value := cookie.String()
	return &value
}

func (s *AnchorAPI) clearRefreshTokenCookie() *string {
	isDev := s.CoreConfig.IsDevelopment()

	//nolint:gosec // Local development needs a non-Secure refresh cookie on localhost.
	cookie := http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     "/v1/auth/refresh",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   !isDev, // false for development, true for production
		SameSite: func() http.SameSite {
			if isDev {
				return http.SameSiteLaxMode
			}
			return http.SameSiteNoneMode
		}(),
	}
	value := cookie.String()
	return &value
}

func (s *AnchorAPI) Login(ctx context.Context, request LoginRequestObject) (
	LoginResponseObject, error,
) {
	loginInput := auth.LoginInput{
		Email:    request.Body.Email,
		Password: request.Body.Password,
	}

	loginResponse, err := s.AuthService.Login(ctx, loginInput)
	if err != nil {
		return nil, err
	}

	responseBody := AuthTokenResponse{
		AccessToken:  loginResponse.AccessToken,
		RefreshToken: loginResponse.RefreshToken,
	}

	response := Login200JSONResponse{
		Body: responseBody,
		Headers: Login200ResponseHeaders{
			SetCookie: s.setRefreshTokenCookie(
				responseBody.RefreshToken, 60*60*24*cookieLifetimeDays,
			),
		},
	}

	return response, nil
}

func (s *AnchorAPI) RefreshToken(
	ctx context.Context, request RefreshTokenRequestObject,
) (RefreshTokenResponseObject, error) {
	if request.Params.RefreshToken == nil {
		return RefreshToken401Response{
			Headers: RefreshToken401ResponseHeaders{
				SetCookie: s.clearRefreshTokenCookie(),
			},
		}, nil
	}
	refreshTokenValue := *request.Params.RefreshToken

	refreshInput := auth.RefreshTokenInput{
		RefreshToken: refreshTokenValue,
	}
	refreshResponse, err := s.AuthService.RefreshToken(ctx, refreshInput)
	if err != nil {
		s.logger.Error().Err(err).Msg("Error refreshing token")
		s.clearRefreshTokenCookie()
		return RefreshToken401Response{
			Headers: RefreshToken401ResponseHeaders{
				SetCookie: s.clearRefreshTokenCookie(),
			},
		}, nil
	}
	responseBody := AuthTokenResponse{
		AccessToken:  refreshResponse.AccessToken,
		RefreshToken: refreshResponse.RefreshToken,
	}

	response := RefreshToken200JSONResponse{
		Body: responseBody,
		Headers: RefreshToken200ResponseHeaders{
			SetCookie: s.setRefreshTokenCookie(
				responseBody.RefreshToken, 60*60*24*cookieLifetimeDays,
			),
		},
	}
	return response, nil
}

func (s *AnchorAPI) Register(
	ctx context.Context, request RegisterRequestObject,
) (RegisterResponseObject, error) {
	registerInput := auth.RegisterInput{
		Email:          request.Body.Email,
		Password:       request.Body.Password,
		InvitationCode: request.Body.InvitationCode,
		TenantName:     request.Body.TenantName,
	}

	_, err := s.AuthService.Register(ctx, registerInput)
	if err != nil {
		return nil, err
	}

	loginInput := auth.LoginInput{
		Email:    registerInput.Email,
		Password: registerInput.Password,
	}
	loginResponse, err := s.AuthService.Login(ctx, loginInput)
	if err != nil {
		s.logger.Error().Err(err).Str(
			"email", registerInput.Email,
		).Msg("Error logging in user immediately after registration")
		return nil, err
	}

	responseBody := AuthTokenResponse{
		AccessToken:  loginResponse.AccessToken,
		RefreshToken: loginResponse.RefreshToken,
	}

	response := Register200JSONResponse{
		Body: responseBody,
		Headers: Register200ResponseHeaders{
			SetCookie: s.setRefreshTokenCookie(
				responseBody.RefreshToken, 60*60*24*cookieLifetimeDays,
			),
		},
	}

	return response, nil
}

func (s *AnchorAPI) Logout(
	_ context.Context, _ LogoutRequestObject,
) (LogoutResponseObject, error) {
	return Logout204Response{
		Headers: Logout204ResponseHeaders{
			SetCookie: s.clearRefreshTokenCookie(),
		},
	}, nil
}
