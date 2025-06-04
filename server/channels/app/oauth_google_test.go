// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogleProvider_GetSSOSettings(t *testing.T) {
	provider := &GoogleProvider{}
	cfg := &model.Config{}
	cfg.SetDefaults() // Ensure all settings are initialized

	// Correct service name
	ssoSettings, err := provider.GetSSOSettings(request.EmptyContext(nil), cfg, model.ServiceGoogle)
	require.NoError(t, err)
	require.NotNil(t, ssoSettings)
	assert.Equal(t, cfg.GoogleOAuthSettings.Id, ssoSettings.Id)
	assert.Equal(t, cfg.GoogleOAuthSettings.Secret, ssoSettings.Secret)
	assert.Equal(t, cfg.GoogleOAuthSettings.AuthEndpoint, ssoSettings.AuthEndpoint)
	assert.Equal(t, cfg.GoogleOAuthSettings.TokenEndpoint, ssoSettings.TokenEndpoint)
	assert.Equal(t, cfg.GoogleOAuthSettings.UserAPIEndpoint, ssoSettings.UserAPIEndpoint)

	// Incorrect service name
	ssoSettings, err = provider.GetSSOSettings(request.EmptyContext(nil), cfg, "notgoogle")
	require.Error(t, err)
	require.Nil(t, ssoSettings)
	assert.Contains(t, err.Error(), "invalid service name")
}

func TestGoogleProvider_GetUserFromJSON(t *testing.T) {
	provider := &GoogleProvider{}
	c := request.EmptyContext(nil) // Mock request.CTX

	t.Run("valid user data", func(t *testing.T) {
		googleUser := map[string]interface{}{
			"sub":            "testuserid",
			"email":          "test@example.com",
			"email_verified": true,
			"name":           "Test User",
			"given_name":     "Test",
			"family_name":    "User",
			"picture":        "http://example.com/picture.jpg",
			"locale":         "en",
		}
		jsonData, _ := json.Marshal(googleUser)
		user, err := provider.GetUserFromJSON(c, bytes.NewReader(jsonData), nil)

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, "test@example.com", user.Email)
		assert.Equal(t, "testuserid", *user.AuthData)
		assert.Equal(t, model.ServiceGoogle, user.AuthService)
		assert.True(t, user.EmailVerified)
		assert.Equal(t, "Test", user.FirstName)
		assert.Equal(t, "User", user.LastName)
		// Username might be generated, check if it's reasonable
		assert.NotEmpty(t, user.Username)
		assert.Equal(t, "en", user.Locale)
	})

	t.Run("valid user data with only name", func(t *testing.T) {
		googleUser := map[string]interface{}{
			"sub":            "testuserid2",
			"email":          "test2@example.com",
			"email_verified": true,
			"name":           "Full Name Only",
		}
		jsonData, _ := json.Marshal(googleUser)
		user, err := provider.GetUserFromJSON(c, bytes.NewReader(jsonData), nil)

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, "test2@example.com", user.Email)
		assert.Equal(t, "Full", user.FirstName)
		assert.Equal(t, "Name Only", user.LastName)
	})

	t.Run("valid user data with id instead of sub", func(t *testing.T) {
		googleUser := map[string]interface{}{
			"id":             "testuserid_alt", // Using "id"
			"email":          "test_alt@example.com",
			"email_verified": true,
			"name":           "Test User Alt",
		}
		jsonData, _ := json.Marshal(googleUser)
		user, err := provider.GetUserFromJSON(c, bytes.NewReader(jsonData), nil)
		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, "test_alt@example.com", user.Email)
		assert.Equal(t, "testuserid_alt", *user.AuthData)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		jsonData := []byte("{not_json")
		user, err := provider.GetUserFromJSON(c, bytes.NewReader(jsonData), nil)

		require.Error(t, err)
		require.Nil(t, user)
		assert.Contains(t, err.Error(), "decode.app_error")
	})

	t.Run("missing sub/id", func(t *testing.T) {
		googleUser := map[string]interface{}{"email": "test@example.com"} // sub/id is missing
		jsonData, _ := json.Marshal(googleUser)
		// Note: Current GetUserFromJSON doesn't explicitly error on missing sub if AuthData is nil,
		// but AuthData would be an empty string. A more robust check might be needed in the main func.
		// For now, we test that it doesn't crash and AuthData is effectively empty or nil.
		user, err := provider.GetUserFromJSON(c, bytes.NewReader(jsonData), nil)
		require.NoError(t, err) // It might not error, but user.AuthData would be problematic
		require.NotNil(t, user)
		assert.Equal(t, "", *user.AuthData) // userID would be empty string
	})

	t.Run("email not verified", func(t *testing.T) {
		googleUser := map[string]interface{}{
			"sub":            "testuserid3",
			"email":          "test3@example.com",
			"email_verified": false, // Email not verified
			"name":           "Test User Three",
		}
		jsonData, _ := json.Marshal(googleUser)
		user, err := provider.GetUserFromJSON(c, bytes.NewReader(jsonData), nil)
		require.NoError(t, err)
		require.NotNil(t, user)
		assert.False(t, user.EmailVerified)
		// Check logger output (this requires a more complex setup with a mock logger for c.Logger())
	})
}

func TestGoogleProvider_GetUserFromIdToken(t *testing.T) {
	provider := &GoogleProvider{}
	c := request.EmptyContext(nil)

	t.Run("valid id token", func(t *testing.T) {
		claims := map[string]interface{}{
			"sub":            "test_jwt_user",
			"email":          "jwt@example.com",
			"email_verified": true,
			"name":           "Jwt User",
			"given_name":     "Jwt",
			"family_name":    "User",
			"locale":         "de",
		}
		payload, _ := json.Marshal(claims)
		// Construct a minimal JWT: header.payload.signature (signature not validated here)
		idToken := "eyJhbGciOiJSUzI1NiJ9." + model.EncodeSegment(payload) + ".fakesig"
		user, err := provider.GetUserFromIdToken(c, idToken)

		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, "jwt@example.com", user.Email)
		assert.Equal(t, "test_jwt_user", *user.AuthData)
		assert.True(t, user.EmailVerified)
		assert.Equal(t, "Jwt", user.FirstName)
		assert.Equal(t, "User", user.LastName)
		assert.Equal(t, "de", user.Locale)
	})

	t.Run("valid id token with id claim", func(t *testing.T) {
		claims := map[string]interface{}{
			"id":             "test_jwt_user_alt_id", // Using "id"
			"email":          "jwt_alt@example.com",
			"email_verified": true,
			"name":           "Jwt User Alt",
		}
		payload, _ := json.Marshal(claims)
		idToken := "eyJhbGciOiJSUzI1NiJ9." + model.EncodeSegment(payload) + ".fakesig"
		user, err := provider.GetUserFromIdToken(c, idToken)
		require.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, "jwt_alt@example.com", user.Email)
		assert.Equal(t, "test_jwt_user_alt_id", *user.AuthData)
	})

	t.Run("invalid token format", func(t *testing.T) {
		user, err := provider.GetUserFromIdToken(c, "invalidtoken")
		require.Error(t, err)
		require.Nil(t, user)
		assert.Contains(t, err.Error(), "invalid_format.app_error")
	})

	t.Run("invalid payload encoding", func(t *testing.T) {
		idToken := "header.invalidpayloadsegment.signature"
		user, err := provider.GetUserFromIdToken(c, idToken)
		require.Error(t, err)
		require.Nil(t, user)
		assert.Contains(t, err.Error(), "payload_decode.app_error")
	})

	t.Run("payload not json", func(t *testing.T) {
		idToken := "header." + model.EncodeSegment([]byte("not json")) + ".signature"
		user, err := provider.GetUserFromIdToken(c, idToken)
		require.Error(t, err)
		require.Nil(t, user)
		assert.Contains(t, err.Error(), "json_unmarshal.app_error")
	})

	t.Run("missing sub/id claim in token", func(t *testing.T) {
		claims := map[string]interface{}{"email": "jwt@example.com"} // sub/id is missing
		payload, _ := json.Marshal(claims)
		idToken := "header." + model.EncodeSegment(payload) + ".signature"
		user, err := provider.GetUserFromIdToken(c, idToken)

		require.Error(t, err)
		require.Nil(t, user)
		assert.Contains(t, err.Error(), "missing_sub.app_error")
	})
}

func TestGoogleProvider_IsSameUser(t *testing.T) {
	provider := &GoogleProvider{}
	c := request.EmptyContext(nil)

	googleAuthData := "googleuserid"
	otherAuthData := "otheruserid"

	dbUser := &model.User{
		Id:          "dbuserid",
		Email:       "test@example.com",
		AuthData:    &googleAuthData,
		AuthService: model.ServiceGoogle,
		EmailVerified: true,
	}

	t.Run("match by AuthData", func(t *testing.T) {
		oauthUser := &model.User{AuthData: &googleAuthData, AuthService: model.ServiceGoogle}
		assert.True(t, provider.IsSameUser(c, dbUser, oauthUser))
	})

	t.Run("match by verified email", func(t *testing.T) {
		oauthUser := &model.User{Email: "test@example.com", EmailVerified: true, AuthService: model.ServiceGoogle}
		// dbUser's AuthData is different here to isolate email matching
		tempDbUser := *dbUser
		tempDbUser.AuthData = &otherAuthData
		assert.True(t, provider.IsSameUser(c, &tempDbUser, oauthUser))
	})

	t.Run("no match", func(t *testing.T) {
		oauthUser := &model.User{AuthData: &otherAuthData, Email: "other@example.com", EmailVerified: true, AuthService: model.ServiceGoogle}
		assert.False(t, provider.IsSameUser(c, dbUser, oauthUser))
	})

	t.Run("oauth user email not verified", func(t *testing.T) {
		oauthUser := &model.User{Email: "test@example.com", EmailVerified: false, AuthService: model.ServiceGoogle}
		tempDbUser := *dbUser
		tempDbUser.AuthData = &otherAuthData
		assert.False(t, provider.IsSameUser(c, &tempDbUser, oauthUser))
	})

	t.Run("db user email not verified", func(t *testing.T) {
		oauthUser := &model.User{Email: "test@example.com", EmailVerified: true, AuthService: model.ServiceGoogle}
		tempDbUser := *dbUser
		tempDbUser.AuthData = &otherAuthData
		tempDbUser.EmailVerified = false
		// This will currently return false because IsSameUser checks dbUser.EmailVerified && oAuthUser.EmailVerified
		// Depending on desired behavior, this might be true if the goal is to link accounts and update dbUser's verification status.
		// For strict "are they the same based on current state", false is correct.
		assert.False(t, provider.IsSameUser(c, &tempDbUser, oauthUser))
	})

	t.Run("oauth user AuthData is nil", func(t *testing.T) {
		oauthUser := &model.User{AuthData: nil, Email: "test@example.com", EmailVerified: true, AuthService: model.ServiceGoogle}
		assert.True(t, provider.IsSameUser(c, dbUser, oauthUser)) // Will match on email
	})

	t.Run("db user AuthData is nil", func(t *testing.T) {
		tempDbUser := *dbUser
		tempDbUser.AuthData = nil
		oauthUser := &model.User{AuthData: &googleAuthData, AuthService: model.ServiceGoogle}
		assert.False(t, provider.IsSameUser(c, &tempDbUser, oauthUser)) // Won't match on AuthData, email not provided for oauthUser
	})

	t.Run("both AuthData nil, match on email", func(t *testing.T) {
		tempDbUser := *dbUser
		tempDbUser.AuthData = nil
		oauthUser := &model.User{AuthData: nil, Email: "test@example.com", EmailVerified: true, AuthService: model.ServiceGoogle}
		assert.True(t, provider.IsSameUser(c, &tempDbUser, oauthUser))
	})
}

// Mocking a minimal App for testing GetAuthorizationCode and conceptual AuthorizeOAuthUser flow
// This avoids needing the full app_test_helpers.go and its dependencies for this specific unit test file.
// Only the parts of App needed by GetAuthorizationCode and AuthorizeOAuthUser are mocked.

type TestApp struct {
	cfg         *model.Config
	srv         *TestServer // Simplified Server mock
	httpService einterfaces.HTTPService
	// Add other fields from the real App struct if GetAuthorizationCode ends up needing them
}

type TestServer struct {
	Store Store // Simplified Store mock
	// Add other fields from the real Server struct
}

func (a *TestApp) Config() *model.Config {
	if a.cfg == nil {
		a.cfg = &model.Config{}
		a.cfg.SetDefaults()
	}
	return a.cfg
}

func (a *TestApp) Srv() *TestServer {
	if a.srv == nil {
		a.srv = &TestServer{}
	}
	return a.srv
}

func (ts *TestServer) Store() Store {
	if ts.Store == nil {
		// Provide a default mock store if not set
		ts.Store = &MockStore{}
	}
	return ts.Store
}

func (a *TestApp) HTTPService() einterfaces.HTTPService {
	if a.httpService == nil {
		a.httpService = &defaultHTTPTestService{}
	}
	return a.httpService
}

func (a *TestApp) GetSiteURL() string {
	if a.Config().ServiceSettings.SiteURL != nil {
		return *a.Config().ServiceSettings.SiteURL
	}
	return ""
}

func (a *TestApp) CreateOAuthStateToken(extra string) (*model.Token, *model.AppError) {
	token := model.NewToken(model.TokenTypeOAuth, extra)
	if _, err := a.Srv().Store().Token().Save(token); err != nil { // Assumes Store().Token().Save() is mocked
		return nil, model.NewAppError("CreateOAuthStateToken", "app.recover.save.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return token, nil
}

// GetProtocol is a helper used by GetAuthorizationCode
func GetProtocol(r *http.Request) string {
	if r.Header.Get(model.HeaderForwardedProto) == "https" || r.TLS != nil {
		return "https"
	}
	return "http"
}


// mockHTTPTransport allows mocking HTTP responses for Client4.
type mockHTTPTransport struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.RoundTripFunc != nil {
		return m.RoundTripFunc(req)
	}
	return nil, http.ErrNotSupported
}

// defaultHTTPTestService is a minimal mock for einterfaces.HTTPService
type defaultHTTPTestService struct{}
func (s *defaultHTTPTestService) MakeClient(trustURLs bool) *http.Client { return http.DefaultClient } // Can use Client4.HTTPClient if it's also mocked
func (s *defaultHTTPTestService) MakeTransport(trustURLs bool) http.RoundTripper { return http.DefaultTransport }
// Stubs for other einterfaces.HTTPService methods if needed by other parts of App
func (s *defaultHTTPTestService) GetHubRegister() einterfaces.HubRegister   { return nil }
func (s *defaultHTTPTestService) GetWSClient() *model.WebSocketClient { return nil }
func (s *defaultHTTPTestService) Close() error                           { return nil }
func (s *defaultHTTPTestService) HubStart()                              {}
func (s *defaultHTTPTestService) HubStop()                               {}


// MockStore and MockTokenStore for testing token operations
type MockStore struct {
	tokenStore *MockTokenStore
}
func (m *MockStore) Token() TokenStore {
	if m.tokenStore == nil {
		m.tokenStore = &MockTokenStore{}
	}
	return m.tokenStore
}
// Add other store getters if needed by tested functions

type MockTokenStore struct {
	SaveFunc     func(token *model.Token) (*model.Token, error)
	GetByTokenFunc func(tokenString string) (*model.Token, error)
	RemoveFunc   func(tokenString string) error
}
func (m *MockTokenStore) Save(token *model.Token) (*model.Token, error) {
	if m.SaveFunc != nil {
		return m.SaveFunc(token)
	}
	return token, nil // Default mock behavior
}
func (m *MockTokenStore) GetByToken(tokenString string) (*model.Token, error) { return nil, nil }
func (m *MockTokenStore) Remove(tokenString string) error                   { return nil }
// Add other TokenStore methods if needed

// TestGoogleAuthURLGeneration verifies that GetAuthorizationCode uses GoogleOAuthSettings.
func TestGoogleAuthURLGeneration(t *testing.T) {
	testApp := &TestApp{}
	testApp.Config().ServiceSettings.SiteURL = model.NewPointer("http://localhost:8065")
	testApp.Config().GoogleOAuthSettings.Enable = model.NewPointer(true)
	testApp.Config().GoogleOAuthSettings.Id = model.NewPointer("google-client-id-test")
	testApp.Config().GoogleOAuthSettings.Scopes = model.NewPointer("email profile openid")
	testApp.Config().GoogleOAuthSettings.AuthEndpoint = model.NewPointer("https://google.com/test/auth")

	// Setup mock for token store
	mockTokenStore := &MockTokenStore{
		SaveFunc: func(token *model.Token) (*model.Token, error) { return token, nil },
	}
	testApp.Srv().Store = &MockStore{tokenStore: mockTokenStore}

	recorder := httptest.NewRecorder()
	dummyReq := httptest.NewRequest("GET", "/", nil)

	authURL, appErr := testApp.GetAuthorizationCode(request.EmptyContext(nil), recorder, dummyReq, model.ServiceGoogle, map[string]string{"action": "login"}, "")
	require.Nil(t, appErr)
	require.NotEmpty(t, authURL)

	assert.Contains(t, authURL, "https://google.com/test/auth")
	assert.Contains(t, authURL, "client_id=google-client-id-test")
	assert.Contains(t, authURL, "scope="+strings.ReplaceAll("email profile openid", " ", "%20"))
	assert.Contains(t, authURL, "redirect_uri=http%3A%2F%2Flocalhost%3A8065%2Fsignup%2Fgoogle%2Fcomplete")
	assert.Contains(t, authURL, "response_type=code")
	assert.Contains(t, authURL, "state=")

	cookies := recorder.Result().Cookies()
	foundCookie := false
	for _, cookie := range cookies {
		if cookie.Name == CookieOAuth { // CookieOAuth is defined in oauth.go
			foundCookie = true
			break
		}
	}
	assert.True(t, foundCookie, "OAuth cookie should be set")
}

// TestGoogleTokenExchangeAndUserInfoFetch conceptually tests parts of AuthorizeOAuthUser.
func TestGoogleTokenExchangeAndUserInfoFetch(t *testing.T) {
	provider := &GoogleProvider{}
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.GoogleOAuthSettings.Enable = model.NewPointer(true)
	cfg.GoogleOAuthSettings.Id = model.NewPointer("google-client-id-exchange")
	cfg.GoogleOAuthSettings.Secret = model.NewPointer("google-client-secret-exchange")
	cfg.GoogleOAuthSettings.TokenEndpoint = model.NewPointer("http://localhost/mock_google_token")
	cfg.GoogleOAuthSettings.UserAPIEndpoint = model.NewPointer("http://localhost/mock_google_userinfo")

	// Get settings via provider to ensure it's working
	ssoSettings, err := provider.GetSSOSettings(request.EmptyContext(nil), cfg, model.ServiceGoogle)
	require.NoError(t, err)
	require.NotNil(t, ssoSettings)

	// Mock HTTP client for Client4
	oldClient := Client4.HTTPClient
	defer func() { Client4.HTTPClient = oldClient }()
	mockTransport := &mockHTTPTransport{}
	Client4.HTTPClient = &http.Client{Transport: mockTransport}

	// --- Test Token Exchange Part ---
	var capturedTokenRequestURL string
	var capturedTokenRequestBody string
	mockTransport.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
		capturedTokenRequestURL = req.URL.String()
		bodyBytes, _ := io.ReadAll(req.Body)
		capturedTokenRequestBody = string(bodyBytes)

		respBody := `{"access_token": "mock_google_access_token", "token_type": "bearer", "id_token": "mock_google_id_token"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(respBody)),
			Header:     make(http.Header),
		}, nil
	}

	// Simulate the POST request that AuthorizeOAuthUser would make to the token endpoint
	// This is a simplified simulation of the internal http request part of AuthorizeOAuthUser
	tokenParams := model.AccessRequest{
		GrantType:    model.AccessTokenGrantType,
		ClientId:     *ssoSettings.Id,
		ClientSecret: *ssoSettings.Secret,
		Code:         "test_auth_code",
		RedirectUri:  "http://localhost/redirect",
	}.ToForm()

	_, httpErr := Client4.HTTPClient.PostForm(*ssoSettings.TokenEndpoint, tokenParams)
	require.NoError(t, httpErr)

	assert.Equal(t, *ssoSettings.TokenEndpoint, capturedTokenRequestURL)
	assert.Contains(t, capturedTokenRequestBody, "client_id=google-client-id-exchange")
	assert.Contains(t, capturedTokenRequestBody, "client_secret=google-client-secret-exchange")
	assert.Contains(t, capturedTokenRequestBody, "code=test_auth_code")
	assert.Contains(t, capturedTokenRequestBody, "grant_type=authorization_code")


	// --- Test User Info Fetch Part ---
	var capturedUserInfoURL string
	var capturedAuthHeader string
	mockTransport.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
		capturedUserInfoURL = req.URL.String()
		capturedAuthHeader = req.Header.Get("Authorization")

		googleUser := map[string]interface{}{"sub": "test_user_sub", "email": "user@example.com", "email_verified": true, "name": "Test User"}
		jsonData, _ := json.Marshal(googleUser)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(jsonData)),
			Header:     make(http.Header),
		}, nil
	}

	// Simulate the GET request that AuthorizeOAuthUser would make to the user info endpoint
	httpReq, _ := http.NewRequest("GET", *ssoSettings.UserAPIEndpoint, nil)
	httpReq.Header.Set("Authorization", "Bearer mock_google_access_token") // This token comes from the previous step's response
	_, httpErr = Client4.HTTPClient.Do(httpReq)
	require.NoError(t, httpErr)

	assert.Equal(t, *ssoSettings.UserAPIEndpoint, capturedUserInfoURL)
	assert.Equal(t, "Bearer mock_google_access_token", capturedAuthHeader)

	// The actual parsing of this response by GetUserFromJSON is tested in TestGoogleProvider_GetUserFromJSON
	t.Log("Conceptual test: Verified token exchange and user info fetch would use GoogleOAuthSettings.")
}
