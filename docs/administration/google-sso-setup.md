# Google Single Sign-On Setup Guide

This guide explains how to configure Google Single Sign-On (SSO) for Mattermost. This allows users to log in to Mattermost using their Google accounts.

## 1. Enable Google SSO in Mattermost

First, you need to enable the Google SSO feature in your Mattermost configuration. This is typically done in `config.json` or via the System Console. The relevant settings are usually found under an `SSOSettings` section, specifically within `GoogleOAuthSettings` (or a similarly named section for Google OAuth).

Ensure the following settings are configured:

*   **Enable**: Set to `true` to enable Google SSO.
*   **Id**: This will be your Google OAuth Client ID.
*   **Secret**: This will be your Google OAuth Client Secret.
*   **AuthURL**: The authorization endpoint URL provided by Google. Defaults to `https://accounts.google.com/o/oauth2/v2/auth`.
*   **TokenURL**: The token endpoint URL provided by Google. Defaults to `https://oauth2.googleapis.com/token`.
*   **UserInfoURL**: The user info endpoint URL provided by Google. Defaults to `https://www.googleapis.com/oauth2/v3/userinfo`.
*   **Scopes**: The scopes to request from Google. Defaults to `profile email`. Common scopes include `openid`, `profile`, and `email`.

**Example `config.json` snippet:**
```json
{
  "GoogleOAuthSettings": {
    "Enable": true,
    "Id": "YOUR_GOOGLE_CLIENT_ID",
    "Secret": "YOUR_GOOGLE_CLIENT_SECRET",
    "AuthURL": "https://accounts.google.com/o/oauth2/v2/auth",
    "TokenURL": "https://oauth2.googleapis.com/token",
    "UserInfoURL": "https://www.googleapis.com/oauth2/v3/userinfo",
    "Scopes": "profile email openid"
  }
}
```
Restart the Mattermost server after making changes to `config.json`.

## 2. Obtain Google OAuth Credentials

To use Google SSO, you need to register your Mattermost instance as an application in the Google Cloud Console.

1.  **Go to Google Cloud Console**: Navigate to [https://console.cloud.google.com/](https://console.cloud.google.com/).
2.  **Create a New Project** (or select an existing one).
3.  **Navigate to APIs & Services > Credentials**:
    *   Click on **Create Credentials** and select **OAuth client ID**.
    *   If prompted, configure the **OAuth consent screen**.
        *   **User Type**: Choose "External" unless you are restricting to users within your Google Workspace organization and have the necessary permissions.
        *   Provide an **App name** (e.g., "Mattermost").
        *   Provide a **User support email**.
        *   For **Authorized domains**, add the domain of your Mattermost instance (e.g., `yourmattermost.com`).
        *   Add developer contact information.
        *   Save and continue.
        *   For **Scopes**, you can leave this blank for now or add basic scopes like `email`, `profile`, `openid`. The scopes configured in Mattermost will take precedence for the actual login flow.
        *   For **Test users**, add any Google accounts that should be able to test the integration before publishing the app (if you chose "External" and your app is in "testing" mode).
4.  **Configure OAuth Client ID**:
    *   Select **Web application** as the Application type.
    *   Give it a **Name** (e.g., "Mattermost Web Client").
    *   Under **Authorized JavaScript origins**, add the base URL of your Mattermost instance (e.g., `https://yourmattermost.com`).
    *   Under **Authorized redirect URIs**, add the Mattermost callback URL. This is typically `https://yourmattermost.com/signup/google/complete` (or your specific Mattermost site URL followed by `/signup/google/complete`).
    *   Click **Create**.
5.  **Note Your Credentials**:
    *   After creation, you will be shown your **Client ID** and **Client Secret**. Copy these values. You will need them for the Mattermost configuration.

**Important Considerations from Google:**
*   Ensure your Authorized Redirect URIs are correct and use `https` for production.
*   Review Google's OAuth 2.0 policies and branding guidelines.
*   If your app remains in "testing" mode in the Google Cloud Console, only registered test users will be able to use the SSO. You may need to "publish" your app for general availability (this usually involves a review process by Google if it's an external-facing app).

## 3. Configure Credentials in Mattermost

Once you have your Client ID and Client Secret from Google:

*   **System Console**:
    1.  Navigate to **Authentication > OpenID Connect (Experimental)** or a similar section for Google/OAuth configuration.
    2.  If there's a specific section for "Google OAuth", use that. Otherwise, you might be using a generic OpenID Connect setup if the new `GoogleOAuthSettings` are exposed under that UI.
    3.  Enter the **Client ID** into the `Id` field (e.g., `GoogleOAuthClientID`).
    4.  Enter the **Client Secret** into the `Secret` field (e.g., `GoogleOAuthClientSecret`).
    5.  Ensure the **AuthURL**, **TokenURL**, **UserInfoURL**, and **Scopes** match what Google expects and what you've configured in `GoogleOAuthSettings`. The defaults are often sufficient.
    6.  Save the settings.

*   **config.json**:
    1.  Open your `config.json` file.
    2.  Locate the `GoogleOAuthSettings` section (or the relevant SSO settings section).
    3.  Set the `Id` field to your Google Client ID.
    4.  Set the `Secret` field to your Google Client Secret.
    5.  Ensure `Enable` is `true`.
    6.  Verify other URLs and scopes if you changed them from the defaults.
    7.  Save the file and restart the Mattermost server.

## 4. Known Limitations & Important Considerations

*   **Scope Configuration**: The `Scopes` field in Mattermost's configuration (`GoogleOAuthSettings`) should align with the scopes your application is authorized to request in the Google Cloud Console. The default `profile email openid` is a common and usually sufficient set.
*   **User Provisioning**: This SSO method primarily handles authentication. User accounts in Mattermost are typically created upon their first successful login with Google if they don't already exist (if user creation is enabled).
*   **Old Google Settings**: If you were previously using a different set of Google SSO settings (e.g., under `GoogleSettings` directly without the "OAuth" suffix in the setting names), ensure you are now using and configuring the new `GoogleOAuthSettings` section for this provider. Migrating users or settings from an old Google SSO integration to this one might require manual steps or specific guidance not covered here.
*   **Error Messages**: Pay attention to error messages on the Mattermost server logs and in the browser console if you encounter issues. They often provide clues about misconfigurations (e.g., redirect URI mismatch, client secret errors).
*   **Google Cloud Console Project State**: If your Google Cloud project or OAuth consent screen is not fully configured or is in a "testing" state, this may restrict who can use the SSO.
*   **API Quotas**: Be mindful of any API usage quotas on the Google Cloud Console side, though standard login operations are unlikely to hit these for most deployments.

By following these steps, you should be able to successfully set up Google Single Sign-On for your Mattermost instance.
