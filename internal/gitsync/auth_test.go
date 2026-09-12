package gitsync

import (
	"slices"
	"strings"
	"testing"

	"github.com/hungtrd/lazytodo/internal/repository"
)

const fakeToken = "github_pat_11ABCDEFG_supersecretvalue"

func TestNewAuthInfersModeFromURL(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want string
	}{
		{"https://github.com/u/todo.git", AuthToken},
		{"http://example.com/u/todo.git", AuthToken},
		{"git@github.com:u/todo.git", AuthSSH},
		{"ssh://git@example.com/u/todo.git", AuthSSH},
		{"", AuthSSH},
	} {
		if got := NewAuth(repository.GitSync{}, tc.url).Mode; got != tc.want {
			t.Fatalf("mode for %q = %q, want %q", tc.url, got, tc.want)
		}
	}

	// An explicit setting overrides the inference.
	if got := NewAuth(repository.GitSync{Auth: AuthSSH}, "https://example.com/x.git").Mode; got != AuthSSH {
		t.Fatalf("explicit auth was ignored: %q", got)
	}
}

func TestAuthCheckRequiresTokenForHTTPS(t *testing.T) {
	t.Setenv(DefaultTokenEnv, "")
	auth := NewAuth(repository.GitSync{}, "https://github.com/u/todo.git")
	err := auth.Check()
	if err == nil {
		t.Fatal("expected a missing-token error")
	}
	if !strings.Contains(err.Error(), DefaultTokenEnv) {
		t.Fatalf("error does not name the env var: %v", err)
	}

	t.Setenv(DefaultTokenEnv, fakeToken)
	if err := NewAuth(repository.GitSync{}, "https://github.com/u/todo.git").Check(); err != nil {
		t.Fatalf("token was set but check failed: %v", err)
	}

	// SSH never needs the variable.
	if err := NewAuth(repository.GitSync{}, "git@github.com:u/todo.git").Check(); err != nil {
		t.Fatalf("ssh auth should not require a token: %v", err)
	}
}

// The whole point of the credential-helper approach is that the token reaches
// git through the environment, never through the command line, where any user
// on the machine could read it out of the process table.
func TestAuthKeepsTokenOutOfArgv(t *testing.T) {
	t.Setenv(DefaultTokenEnv, fakeToken)
	auth := NewAuth(repository.GitSync{}, "https://github.com/u/todo.git")

	args := auth.Args()
	if len(args) != 4 {
		t.Fatalf("args = %v, want two -c flags", args)
	}
	for _, arg := range args {
		if strings.Contains(arg, fakeToken) {
			t.Fatalf("token leaked into argv: %q", arg)
		}
	}
	// The first helper is empty so an inherited keychain helper cannot
	// intercept the request or persist the token.
	if args[0] != "-c" || args[1] != "credential.helper=" {
		t.Fatalf("helper chain is not reset first: %v", args)
	}
	if !strings.Contains(args[3], childTokenEnv) || !strings.Contains(args[3], childUserEnv) {
		t.Fatalf("helper does not read the token from the environment: %q", args[3])
	}

	env := auth.Env()
	if !slices.Contains(env, "GIT_TERMINAL_PROMPT=0") {
		t.Fatalf("env does not disable interactive prompts: %v", env)
	}
	if !slices.Contains(env, childTokenEnv+"="+fakeToken) {
		t.Fatalf("token was not passed through the environment: %v", env)
	}
	if !slices.Contains(env, childUserEnv+"="+DefaultTokenUser) {
		t.Fatalf("token username missing: %v", env)
	}
}

func TestAuthSSHAddsNoCredentialArgs(t *testing.T) {
	t.Setenv(DefaultTokenEnv, fakeToken)
	auth := NewAuth(repository.GitSync{}, "git@github.com:u/todo.git")
	if args := auth.Args(); len(args) != 0 {
		t.Fatalf("ssh auth added credential args: %v", args)
	}
	for _, entry := range auth.Env() {
		if strings.Contains(entry, fakeToken) {
			t.Fatalf("ssh auth exported the token: %q", entry)
		}
	}
}

func TestAuthUsesCustomTokenEnvAndUser(t *testing.T) {
	t.Setenv("MY_TOKEN", fakeToken)
	auth := NewAuth(repository.GitSync{TokenEnv: "MY_TOKEN", TokenUser: "oauth2"}, "https://gitlab.com/u/todo.git")
	if err := auth.Check(); err != nil {
		t.Fatal(err)
	}
	// The helper string stays a constant; only the value moves.
	if !slices.Contains(auth.Env(), childUserEnv+"=oauth2") {
		t.Fatalf("custom token user was not applied: %v", auth.Env())
	}
	if !slices.Contains(auth.Env(), childTokenEnv+"="+fakeToken) {
		t.Fatalf("custom env var was not read: %v", auth.Env())
	}
}

func TestAuthScrubAndDescribeNeverRevealTheToken(t *testing.T) {
	t.Setenv(DefaultTokenEnv, fakeToken)
	auth := NewAuth(repository.GitSync{}, "https://github.com/u/todo.git")

	scrubbed := auth.Scrub("fatal: could not read https://x-access-token:" + fakeToken + "@github.com")
	if strings.Contains(scrubbed, fakeToken) {
		t.Fatalf("scrub left the token in place: %q", scrubbed)
	}
	if !strings.Contains(scrubbed, "***") {
		t.Fatalf("scrub did not mask the token: %q", scrubbed)
	}

	described := auth.Describe()
	if strings.Contains(described, fakeToken) {
		t.Fatalf("describe leaked the token: %q", described)
	}
	if !strings.Contains(described, "set") {
		t.Fatalf("describe = %q, want it to report the env var state", described)
	}

	t.Setenv(DefaultTokenEnv, "")
	if got := NewAuth(repository.GitSync{}, "https://github.com/u/todo.git").Describe(); !strings.Contains(got, "not set") {
		t.Fatalf("describe = %q, want it to report a missing token", got)
	}
}
