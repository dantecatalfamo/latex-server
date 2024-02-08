package client

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
)

func Login(globalConfig GlobalConfig, username, password string) error {
	loginUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, "login")
	if err != nil {
		return fmt.Errorf("Login create path: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Errorf("Login get hostname: %w", err)
	}

	form := url.Values{}
	form["username"] = []string{ username }
	form["password"] = []string{ password }
	form["description"] = []string{ fmt.Sprintf("%s - %s", runtime.GOOS, hostname) }

	resp, err := http.PostForm(loginUrl, form)
	if err != nil {
		return fmt.Errorf("Login post form: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrIncorrectLogin
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("Login unexpected status code %d", resp.StatusCode)
	}

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Login read body: %w", err)
	}

	globalConfig.Token = strings.Trim(string(token), "\n")
	globalConfig.User = username

	if err := WriteGlobalConfig(globalConfig); err != nil {
		return fmt.Errorf("Login write config: %w", err)
	}

	return nil
}

var ErrIncorrectLogin = errors.New("incorrect username or password")

func Logout(globalConfig GlobalConfig) error {
	logoutUrl, err := url.JoinPath(globalConfig.ServerBaseUrl, "logout")
	if err != nil {
		return fmt.Errorf("Logout create url: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, logoutUrl, nil)
	if err != nil {
		return fmt.Errorf("Logout create request: %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", globalConfig.Token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Logout do request: %w", err)
	}
	defer resp.Body.Close()

	// Unauthorized is invalid auth token, we can clear it and return success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("Logout unexpected status code %d", resp.StatusCode)
	}

	globalConfig.Token = ""
	globalConfig.User = ""

	if err := WriteGlobalConfig(globalConfig); err != nil {
		return fmt.Errorf("Logout write global config: %w", err)
	}

	return nil
}
