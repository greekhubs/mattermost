package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/mattermost/mattermost/server/public/plugin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const pluginID = "com.mattermost.google-oauth"

// Plugin implements the interface expected by the Mattermost server.
type Plugin struct {
	plugin.MattermostPlugin
	oauthConfig *oauth2.Config
}

func (p *Plugin) OnActivate() error {
	siteURL := p.API.GetConfig().ServiceSettings.SiteURL
	if siteURL == nil || *siteURL == "" {
		return fmt.Errorf("site URL must be set")
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set")
	}

	p.oauthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  fmt.Sprintf("%s/plugins/%s/oauth/complete", *siteURL, pluginID),
		Scopes:       []string{"profile", "email"},
		Endpoint:     google.Endpoint,
	}

	return nil
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/login":
		url := p.oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOffline)
		http.Redirect(w, r, url, http.StatusFound)
	case "/oauth/complete":
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		token, err := p.oauthConfig.Exchange(r.Context(), code)
		if err != nil {
			p.API.LogError("oauth exchange failed", "err", err.Error())
			http.Error(w, "oauth failed", http.StatusInternalServerError)
			return
		}
		p.API.LogInfo("received oauth token", "access", token.AccessToken)
		_, _ = w.Write([]byte("Google OAuth login successful"))
	default:
		http.NotFound(w, r)
	}
}
