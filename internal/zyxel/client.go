package zyxel

import (
	"context"
	"crypto/md5" //nolint:gosec // required by the firmware.
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxBodyBytes = 4 << 20
	rsaExponent  = 0x10001

	loginPollInterval = 500 * time.Millisecond
	loginPollAttempts = 10
)

var ErrAuth = errors.New("authentication failed")

type Client struct {
	http   *http.Client
	base   *url.URL
	secure bool
	pass   string

	mu       sync.Mutex
	loggedIn bool
	xsrf     string
}

func New(baseURL, password string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid switch url %q", baseURL)
	}
	if password == "" {
		return nil, errors.New("empty password")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if httpClient.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, err
		}
		httpClient.Jar = jar
	}
	return &Client{http: httpClient, base: u, secure: u.Scheme == "https", pass: password}, nil
}

type envelope struct {
	Data   json.RawMessage `json:"data"`
	XSRF   string          `json:"xsrfToken"`
	Logout bool            `json:"logout"`
	Reason string          `json:"reason"`
}

func Get[T any](ctx context.Context, c *Client, cmd string) (T, error) {
	var out T
	raw, err := c.get(ctx, cmd)
	if err != nil {
		return out, fmt.Errorf("%s: %w", cmd, err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("%s: decode: %w", cmd, err)
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, cmd string) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for attempt := 0; ; attempt++ {
		if !c.loggedIn {
			if err := c.login(ctx); err != nil {
				return nil, fmt.Errorf("login: %w", err)
			}
		}
		var env envelope
		if err := c.do(ctx, http.MethodGet, "get", cmd, "", &env); err != nil {
			return nil, err
		}
		if env.Logout {
			c.loggedIn = false
			if attempt == 0 {
				continue
			}
			return nil, fmt.Errorf("session rejected: %s", env.Reason)
		}
		if env.XSRF != "" {
			c.xsrf = env.XSRF
		}
		return env.Data, nil
	}
}

func (c *Client) Logout(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loggedIn {
		return nil
	}
	c.loggedIn = false
	var out json.RawMessage
	return c.do(ctx, http.MethodPost, "set", "home_logout", form("xsrfToken", c.xsrf), &out)
}

func (c *Client) login(ctx context.Context) error {
	var info envelope
	if err := c.do(ctx, http.MethodGet, "get", "home_loginInfo", "", &info); err != nil {
		return err
	}
	var li struct {
		Modulus string `json:"modulus"`
	}
	if err := json.Unmarshal(info.Data, &li); err != nil {
		return fmt.Errorf("decode login info: %w", err)
	}

	password := c.pass
	if !c.secure {
		var err error
		if password, err = encryptPassword(li.Modulus, c.pass); err != nil {
			return err
		}
	}

	xsrf, err := randomHex(8)
	if err != nil {
		return err
	}
	var auth struct {
		Status string `json:"status"`
		AuthID string `json:"authId"`
	}
	if err := c.do(ctx, http.MethodPost, "set", "home_loginAuth", form("password", password, "xsrfToken", xsrf), &auth); err != nil {
		return err
	}
	if auth.AuthID == "" {
		return fmt.Errorf("%w: no auth id (status %q)", ErrAuth, auth.Status)
	}

	for range loginPollAttempts {
		var st struct {
			Data struct {
				Status     string `json:"status"`
				FailReason string `json:"failReason"`
			} `json:"data"`
			XSRF string `json:"xsrfToken"`
		}
		if err := c.do(ctx, http.MethodPost, "set", "home_loginStatus", form("authId", auth.AuthID, "xsrfToken", xsrf), &st); err != nil {
			return err
		}
		switch st.Data.Status {
		case "ok":
			c.loggedIn = true
			c.xsrf = st.XSRF
			if c.xsrf == "" {
				c.xsrf = xsrf
			}
			return nil
		case "fail":
			return fmt.Errorf("%w: %s", ErrAuth, strings.TrimSpace(st.Data.FailReason))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(loginPollInterval):
		}
	}
	return errors.New("login still pending after polling")
}

func (c *Client) do(ctx context.Context, method, kind, cmd, body string, out any) error {
	query := "cmd=" + url.QueryEscape(cmd) + "&dummy=" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	sum := md5.Sum([]byte(query)) //nolint:gosec // see import.
	u := *c.base
	u.Path = "/cgi/" + kind + ".cgi"
	u.RawQuery = query + "&bj4=" + hex.EncodeToString(sum[:])

	var rd io.Reader
	if method == http.MethodPost {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func form(kv ...string) string {
	var b strings.Builder
	b.WriteString("_ds=1")
	for i := 0; i+1 < len(kv); i += 2 {
		b.WriteString("&" + kv[i] + "=" + url.QueryEscape(kv[i+1]))
	}
	b.WriteString("&_de=1")
	return b.String()
}

func encryptPassword(modulusHex, password string) (string, error) {
	n, ok := new(big.Int).SetString(strings.TrimSpace(modulusHex), 16)
	if !ok || n.Sign() <= 0 {
		return "", fmt.Errorf("invalid rsa modulus %q", modulusHex)
	}
	//nolint:staticcheck // PKCS#1 v1.5 is what the firmware login expects.
	enc, err := rsa.EncryptPKCS1v15(rand.Reader, &rsa.PublicKey{N: n, E: rsaExponent}, []byte(password))
	if err != nil {
		return "", fmt.Errorf("encrypt password: %w", err)
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
