package zyxeltest

import (
	"crypto/md5" //nolint:gosec // required by the firmware.
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const Password = "test-password"

type Server struct {
	*httptest.Server

	key     *rsa.PrivateKey
	results map[string]string

	mu     sync.Mutex
	epoch  int
	logins int
}

func NewServer(t testing.TB, results map[string]string) *Server {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	s := &Server{key: key, results: results}
	mux := http.NewServeMux()
	mux.HandleFunc("/cgi/get.cgi", s.handleGet)
	mux.HandleFunc("/cgi/set.cgi", s.handleSet)
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func (s *Server) RevokeSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epoch++
}

func (s *Server) Logins() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logins
}

func (s *Server) session() string { return fmt.Sprintf("session-%d", s.epoch) }

func (s *Server) authenticated(r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := r.Cookie("HTTP_SESSID")
	return err == nil && c.Value == s.session()
}

func validSignature(r *http.Request) (string, bool) {
	raw := r.URL.RawQuery
	i := strings.LastIndex(raw, "&bj4=")
	if i < 0 {
		return "", false
	}
	sum := md5.Sum([]byte(raw[:i])) //nolint:gosec // see import.
	if raw[i+len("&bj4="):] != hex.EncodeToString(sum[:]) {
		return "", false
	}
	return r.URL.Query().Get("cmd"), true
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	cmd, ok := validSignature(r)
	if !ok {
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}
	if cmd == "home_loginInfo" {
		modulus := strings.ToUpper(s.key.N.Text(16)) + "\n"
		_, _ = fmt.Fprintf(w, `{"data":{"model_name":"XGS1250-12","logined":"0","modulus":%q},"xsrfToken":""}`, modulus)
		return
	}
	if !s.authenticated(r) {
		_, _ = fmt.Fprint(w, `{"logout":true,"reason":"notAuth"}`)
		return
	}
	data, ok := s.results[cmd]
	if !ok {
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}
	_, _ = fmt.Fprintf(w, `{"data":%s,"xsrfToken":"0123456789abcdef"}`, data)
}

func (s *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	cmd, ok := validSignature(r)
	if !ok {
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch cmd {
	case "home_loginAuth":
		authID := "rejected"
		if s.passwordOK(r.PostForm) {
			authID = "granted"
		}
		_, _ = fmt.Fprintf(w, `{"status":"ok","authId":%q}`, authID)
	case "home_loginStatus":
		if r.PostForm.Get("authId") != "granted" {
			_, _ = fmt.Fprint(w, `{"data":{"status":"fail","failReason":"\"Wrong password\""},"xsrfToken":""}`)
			return
		}
		s.mu.Lock()
		s.logins++
		session := s.session()
		s.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "HTTP_SESSID", Value: session, Path: "/"})
		_, _ = fmt.Fprint(w, `{"data":{"status":"ok"},"xsrfToken":"0011223344556677"}`)
	case "home_logout":
		s.RevokeSessions()
		_, _ = fmt.Fprint(w, `{"data":{},"xsrfToken":""}`)
	default:
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
	}
}

func (s *Server) passwordOK(form url.Values) bool {
	enc, err := base64.StdEncoding.DecodeString(form.Get("password"))
	if err != nil {
		return false
	}
	//nolint:staticcheck // mirrors the firmware.
	plain, err := rsa.DecryptPKCS1v15(rand.Reader, s.key, enc)
	return err == nil && string(plain) == Password
}
