package api

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ApiConfig holds the configuration needed to create an API client
type ApiConfig struct {
	AuthOverride string   // Auth string from --auth flag
	Selected     string   // Selected email from config
	Credentials  []string // All stored credentials
	Proxy        string   // Proxy URL
	Saver        bool     // Use storage-saver quality
	UseQuota     bool     // Files count against quota
}

// Api represents a Google Photos API client
type Api struct {
	AndroidAPIVersion int64
	Model             string
	Make              string
	ClientVersionCode int64
	UserAgent         string
	Language          string
	AuthData          string
	Client            *http.Client
	authTokenCache    map[string]string
	saver             bool
	useQuota          bool
}

// NewApi creates a new Google Photos API client with the given configuration
func NewApi(cfg ApiConfig) (*Api, error) {
	var credentials string
	var language string

	// Check for auth override from --auth flag
	if cfg.AuthOverride != "" {
		credentials = cfg.AuthOverride
		params, err := url.ParseQuery(cfg.AuthOverride)
		if err == nil {
			language = params.Get("lang")
		}
	} else {
		// Use credentials from config file
		selectedEmail := cfg.Selected
		if len(selectedEmail) == 0 {
			return nil, fmt.Errorf("no account is selected")
		}
		for _, c := range cfg.Credentials {
			params, err := url.ParseQuery(c)
			if err != nil {
				continue
			}
			if params.Get("Email") == selectedEmail {
				credentials = c
				language = params.Get("lang")
			}
		}
		if len(credentials) == 0 {
			return nil, fmt.Errorf("no credentials with matching selected email found")
		}
	}

	client, err := NewHTTPClientWithProxy(cfg.Proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	api := &Api{
		AndroidAPIVersion: 28,
		Model:             "Pixel XL",
		Make:              "Google",
		ClientVersionCode: 49029607,
		Language:          language,
		AuthData:          strings.TrimSpace(credentials),
		Client:            client,
		authTokenCache: map[string]string{
			"Expiry": "0",
			"Auth":   "",
		},
		saver:    cfg.Saver,
		useQuota: cfg.UseQuota,
	}

	api.UserAgent = fmt.Sprintf(
		"com.google.android.apps.photos/%d (Linux; U; Android 9; %s; %s; Build/PQ2A.190205.001; Cronet/127.0.6510.5) (gzip)",
		api.ClientVersionCode,
		api.Language,
		api.Model,
	)

	return api, nil
}

// BearerToken returns a valid bearer token, refreshing if necessary
func (a *Api) BearerToken() (string, error) {
	expiryStr := a.authTokenCache["Expiry"]
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid expiry time: %w", err)
	}

	if expiry <= time.Now().Unix() {
		resp, err := a.refreshAuthToken()
		if err != nil {
			return "", fmt.Errorf("failed to get auth token: %w", err)
		}
		a.authTokenCache = resp
	}

	if token, ok := a.authTokenCache["Auth"]; ok && token != "" {
		return token, nil
	}

	return "", errors.New("auth response does not contain bearer token")
}

// refreshAuthToken fetches a new auth token from Google
func (a *Api) refreshAuthToken() (map[string]string, error) {
	authDataValues, err := url.ParseQuery(a.AuthData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth data: %w", err)
	}

	authRequestData := url.Values{
		"androidId":                    {authDataValues.Get("androidId")},
		"app":                          {"com.google.android.apps.photos"},
		"client_sig":                   {authDataValues.Get("client_sig")},
		"callerPkg":                    {"com.google.android.apps.photos"},
		"callerSig":                    {authDataValues.Get("callerSig")},
		"device_country":               {authDataValues.Get("device_country")},
		"Email":                        {authDataValues.Get("Email")},
		"google_play_services_version": {authDataValues.Get("google_play_services_version")},
		"lang":                         {authDataValues.Get("lang")},
		"oauth2_foreground":            {authDataValues.Get("oauth2_foreground")},
		"sdk_version":                  {authDataValues.Get("sdk_version")},
		"service":                      {authDataValues.Get("service")},
		"Token":                        {authDataValues.Get("Token")},
	}

	headers := map[string]string{
		"Accept-Encoding": "gzip",
		"app":             "com.google.android.apps.photos",
		"Connection":      "Keep-Alive",
		"Content-Type":    "application/x-www-form-urlencoded",
		"device":          authRequestData.Get("androidId"),
		"User-Agent":      "GoogleAuth/1.4 (Pixel XL PQ2A.190205.001); gzip",
	}

	req, err := http.NewRequest(
		"POST",
		"https://android.googleapis.com/auth",
		strings.NewReader(authRequestData.Encode()),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth request failed after retries: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return make(map[string]string), fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Handle gzip encoding if present
	var reader io.Reader
	reader, err = gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.(*gzip.Reader).Close()

	// Parse the response body
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the key=value response format
	parsedAuthResponse := make(map[string]string)
	for _, line := range strings.Split(string(bodyBytes), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			parsedAuthResponse[parts[0]] = parts[1]
		}
	}

	// Validate we got the required fields
	if parsedAuthResponse["Auth"] == "" {
		return nil, errors.New("auth response missing Auth token")
	}
	if parsedAuthResponse["Expiry"] == "" {
		return nil, errors.New("auth response missing Expiry")
	}

	return parsedAuthResponse, nil
}

// CommonHeaders returns the standard headers used for Google Photos API requests
func (a *Api) CommonHeaders(bearerToken string) map[string]string {
	return map[string]string{
		"Accept-Encoding":          "gzip",
		"Accept-Language":          a.Language,
		"Content-Type":             "application/x-protobuf",
		"User-Agent":               a.UserAgent,
		"Authorization":            "Bearer " + bearerToken,
		"x-goog-ext-173412678-bin": "CgcIAhClARgC",
		"x-goog-ext-174067345-bin": "CgIIAg==",
	}
}

// DeviceInfo returns the current device model and make info
func (a *Api) DeviceInfo() (model, make string, apiVersion int64) {
	return a.Model, a.Make, a.AndroidAPIVersion
}

// SetModel updates the device model (used for quality settings)
func (a *Api) SetModel(model string) {
	a.Model = model
}
