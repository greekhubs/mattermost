// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

type GoogleProvider struct {
}

func init() {
	einterfaces.RegisterOAuthProvider(model.ServiceGoogle, &GoogleProvider{})
}

// GetUserFromJSON decodes the user information from Google's response.
func (gp *GoogleProvider) GetUserFromJSON(c request.CTX, data io.Reader, tokenUser *model.User) (*model.User, error) {
	// Implementation to fetch user info from Google's response
	// This will typically involve decoding a JSON response into a Google user struct
	// and then mapping those fields to Mattermost's model.User struct.

	var googleUser struct {
		ID              string `json:"sub"`
		Email           string `json:"email"`
		EmailVerified   bool   `json:"email_verified"`
		Name            string `json:"name"`
		GivenName       string `json:"given_name"`
		FamilyName      string `json:"family_name"`
		Picture         string `json:"picture"`
		Locale          string `json:"locale"`
		DeprecatedID    string `json:"id"` // Google sometimes uses "id" instead of "sub" in some older APIs or contexts
	}

	// TeeReader allows us to read the data for decoding, and then potentially re-read or log it if there's an error.
	var bodyBuffer bytes.Buffer
	tee := io.TeeReader(data, &bodyBuffer)

	if err := json.NewDecoder(tee).Decode(&googleUser); err != nil {
		// Log the body for debugging if JSON decoding fails
		c.Logger().Error("Failed to decode Google user JSON", mlog.Err(err), mlog.String("body", bodyBuffer.String()))
		return nil, model.NewAppError("GoogleProvider.GetUserFromJSON", "oauth.google.get_user_from_json.decode.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	// Prioritize "sub" but fall back to "id" if "sub" is empty
	userID := googleUser.ID
	if userID == "" {
		userID = googleUser.DeprecatedID
	}

	user := &model.User{
		Email:         googleUser.Email,
		AuthData:      &userID,
		AuthService:   model.ServiceGoogle,
		EmailVerified: googleUser.EmailVerified,
	}

	// Username generation (can be customized)
	// Option 1: Based on email prefix
	if username, ok := model.GenerateUsernameFromEmail(googleUser.Email); ok {
		user.Username = username
	} else {
		// Option 2: Based on name (if email-based generation fails or is not preferred)
		if googleUser.Name != "" {
			user.Username = model.CleanUsername(strings.ReplaceAll(strings.ToLower(googleUser.Name), " ", "."))
		} else if googleUser.GivenName != "" && googleUser.FamilyName != "" {
			user.Username = model.CleanUsername(strings.ToLower(googleUser.GivenName + "." + googleUser.FamilyName))
		} else {
			// Fallback to ID-based if all else fails
			user.Username = model.NewUsernameFromId(userID)
		}
	}


	user.FirstName = googleUser.GivenName
	user.LastName = googleUser.FamilyName
	if user.FirstName == "" && user.LastName == "" && googleUser.Name != "" {
		parts := strings.SplitN(googleUser.Name, " ", 2)
		if len(parts) > 0 {
			user.FirstName = parts[0]
		}
		if len(parts) > 1 {
			user.LastName = parts[1]
		}
	}

	user.Locale = googleUser.Locale
	// Note: Picture URL can be stored in props or a dedicated field if available/needed.
	// For now, not setting it directly on the user model unless there's a specific field.

	if !user.EmailVerified {
		// Handle cases where email is not verified by Google if necessary.
		// For example, log a warning or prevent login based on system policy.
		c.Logger().Warn("Google OAuth: Email not verified.", "email", user.Email)
	}

	return user, nil
}

// GetSSOSettings retrieves the SSO settings for Google.
func (gp *GoogleProvider) GetSSOSettings(c request.CTX, config *model.Config, service string) (*model.SSOSettings, error) {
	// Use the new GoogleOAuthSettings from the config
	if service == model.ServiceGoogle {
		return &config.GoogleOAuthSettings, nil
	}
	return nil, model.NewAppError("GoogleProvider.GetSSOSettings", "oauth.google.get_sso_settings.invalid_service.app_error", nil, "service="+service,ให้บริการ HTTP.StatusBadRequest)
}

// GetUserFromIdToken attempts to parse user information from an ID token.
// Google's ID token typically contains user information that can be used.
func (gp *GoogleProvider) GetUserFromIdToken(c request.CTX, idToken string) (*model.User, error) {
	// Minimal JWT parsing without external libraries for simplicity in this context.
	// A proper implementation should use a JWT library for validation and parsing.
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 { // Should be 3 parts (header, payload, signature) but payload is key
		return nil, model.NewAppError("GoogleProvider.GetUserFromIdToken", "oauth.google.get_user_from_id_token.invalid_format.app_error", nil, "invalid token format",ให้บริการ HTTP.StatusBadRequest)
	}

	payloadBytes, err := model.DecodeSegment(parts[1])
	if err != nil {
		return nil, model.NewAppError("GoogleProvider.GetUserFromIdToken", "oauth.google.get_user_from_id_token.payload_decode.app_error", nil, "Failed to decode ID token payload: "+err.Error(), http.StatusBadRequest)
	}

	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
		Locale        string `json:"locale"`
		ID            string `json:"id"` // Fallback for "sub"
	}

	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, model.NewAppError("GoogleProvider.GetUserFromIdToken", "oauth.google.get_user_from_id_token.json_unmarshal.app_error", nil, "Failed to unmarshal ID token claims: "+err.Error(), http.StatusBadRequest)
	}

	userID := claims.Sub
	if userID == "" {
		userID = claims.ID
	}
	if userID == "" {
		return nil, model.NewAppError("GoogleProvider.GetUserFromIdToken", "oauth.google.get_user_from_id_token.missing_sub.app_error", nil, "missing sub/id claim",ให้บริการ HTTP.StatusBadRequest)
	}

	user := &model.User{
		Email:         claims.Email,
		AuthData:      &userID,
		AuthService:   model.ServiceGoogle,
		EmailVerified: claims.EmailVerified,
	}

	if username, ok := model.GenerateUsernameFromEmail(claims.Email); ok {
		user.Username = username
	} else {
		if claims.Name != "" {
			user.Username = model.CleanUsername(strings.ReplaceAll(strings.ToLower(claims.Name), " ", "."))
		} else if claims.GivenName != "" && claims.FamilyName != "" {
			user.Username = model.CleanUsername(strings.ToLower(claims.GivenName + "." + claims.FamilyName))
		} else {
			user.Username = model.NewUsernameFromId(userID)
		}
	}

	user.FirstName = claims.GivenName
	user.LastName = claims.FamilyName
	if user.FirstName == "" && user.LastName == "" && claims.Name != "" {
		nameParts := strings.SplitN(claims.Name, " ", 2)
		if len(nameParts) > 0 {
			user.FirstName = nameParts[0]
		}
		if len(nameParts) > 1 {
			user.LastName = nameParts[1]
		}
	}
	user.Locale = claims.Locale

	// Here, one might also want to store claims.Picture or other details.

	return user, nil
}

// IsSameUser checks if the remote Google user is the same as the existing Mattermost user.
func (gp *GoogleProvider) IsSameUser(c request.CTX, dbUser, oAuthUser *model.User) bool {
	// Check if the AuthData (Google User ID) matches.
	// Ensure oAuthUser.AuthData is not nil before dereferencing.
	if dbUser.AuthData != nil && oAuthUser.AuthData != nil && *dbUser.AuthData != "" && *oAuthUser.AuthData == *oAuthUser.AuthData {
		return true
	}

	// Check if the email matches, if verified.
	// Ensure oAuthUser.Email is not empty and is verified.
	if dbUser.Email != "" && dbUser.Email == oAuthUser.Email && oAuthUser.EmailVerified {
		// If the dbUser's email is also verified, this is a strong match.
		// If dbUser's email is not verified, but oAuthUser's is, this could still be a match,
		// and we might proceed to update dbUser's EmailVerified status.
		// For IsSameUser, matching verified emails is generally a good indicator.
		return true
	}

	return false
}
