package gitsync

import (
	"fmt"
	"os"
	"strings"

	"github.com/hungtrd/lazytodo/internal/repository"
)

const (
	// AuthSSH lets git use whatever SSH agent and keys the user already has.
	AuthSSH = "ssh"
	// AuthToken authenticates over HTTPS with a personal access token.
	AuthToken = "token"

	DefaultTokenEnv  = "LAZYTODO_GIT_TOKEN"
	DefaultTokenUser = "x-access-token"

	// childTokenEnv and childUserEnv are the fixed names the credential helper
	// reads inside the git child process. Keeping them constant lets the helper
	// string stay a literal even when the user points TokenEnv elsewhere.
	childTokenEnv = "LAZYTODO_GIT_TOKEN"
	childUserEnv  = "LAZYTODO_GIT_USER"
)

// credentialHelper feeds the token to git without ever writing it to disk.
//
// The token value is not in this string, only the names of environment
// variables that the shell expands inside the git child process, so it never
// reaches the process table either. The two alternatives were both rejected:
// embedding the token in the remote URL leaves it in .git/config in plain
// text, and -c http.extraHeader puts it straight into argv.
const credentialHelper = `!f() { echo "username=${` + childUserEnv + `}"; echo "password=${` + childTokenEnv + `}"; }; f`

// Auth resolves how git should authenticate for a given remote.
type Auth struct {
	Mode      string
	TokenUser string
	TokenEnv  string
	token     string
}

// NewAuth picks the mode from the config, falling back to the remote URL
// scheme: HTTPS means a token, anything else means SSH.
func NewAuth(cfg repository.GitSync, remoteURL string) Auth {
	mode := strings.TrimSpace(strings.ToLower(cfg.Auth))
	if mode == "" {
		mode = inferAuthMode(remoteURL)
	}
	tokenEnv := cfg.TokenEnv
	if tokenEnv == "" {
		tokenEnv = DefaultTokenEnv
	}
	tokenUser := cfg.TokenUser
	if tokenUser == "" {
		tokenUser = DefaultTokenUser
	}
	auth := Auth{Mode: mode, TokenUser: tokenUser, TokenEnv: tokenEnv}
	if mode == AuthToken {
		auth.token = strings.TrimSpace(os.Getenv(tokenEnv))
	}
	return auth
}

func inferAuthMode(remoteURL string) string {
	url := strings.TrimSpace(strings.ToLower(remoteURL))
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return AuthToken
	}
	return AuthSSH
}

// HasToken reports whether the configured environment variable is populated.
// It never exposes the value itself.
func (a Auth) HasToken() bool { return a.token != "" }

// Check returns an actionable error when token auth is selected but the
// environment variable is missing.
func (a Auth) Check() error {
	if a.Mode == AuthToken && a.token == "" {
		return fmt.Errorf(
			"%s is not set; export a personal access token before syncing over HTTPS, "+
				"or use an SSH remote instead", a.TokenEnv)
	}
	return nil
}

// Args returns the git -c flags for network commands. The first empty helper
// resets any inherited helper chain so the system keychain neither intercepts
// the request nor stores the token.
func (a Auth) Args() []string {
	if a.Mode != AuthToken || a.token == "" {
		return nil
	}
	return []string{"-c", "credential.helper=", "-c", "credential.helper=" + credentialHelper}
}

// Env returns the extra environment for the git child process. The token value
// travels here rather than in argv.
func (a Auth) Env() []string {
	env := []string{"GIT_TERMINAL_PROMPT=0"}
	if a.Mode == AuthToken && a.token != "" {
		env = append(env,
			childTokenEnv+"="+a.token,
			childUserEnv+"="+a.TokenUser,
		)
	}
	return env
}

// Scrub removes the token from text that is about to be logged or shown, in
// case git echoed back a URL that carried credentials.
func (a Auth) Scrub(text string) string {
	if a.token == "" {
		return text
	}
	return strings.ReplaceAll(text, a.token, "***")
}

// Describe renders the auth mode for `sync status` without leaking the token.
func (a Auth) Describe() string {
	if a.Mode != AuthToken {
		return AuthSSH
	}
	state := "not set"
	if a.token != "" {
		state = "set"
	}
	return fmt.Sprintf("token (%s: %s)", a.TokenEnv, state)
}
