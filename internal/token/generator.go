package token

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Token struct {
	Token string `json:"token"`
}

func GetToken(url url.URL) string {
	clnt := &http.Client{ //nolint:exhaustruct
		Timeout: time.Second * 10, //nolint:mnd
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url.String(), nil)
	if err != nil {
		return ""
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := clnt.Do(req)
	if err != nil {
		return ""
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	rawToken, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	token := Token{Token: ""}

	err = json.Unmarshal(rawToken, &token)
	if err != nil {
		return ""
	}

	return token.Token
}
