package config

import "testing"

func TestResolvePrecedence_FlagBeatsEnvBeatsProfile(t *testing.T) {
	f := &File{
		Profiles: map[string]Profile{
			"lab": {Server: "https://profile.example", Auth: AuthConfig{Type: "token", Secret: "p"}},
		},
	}
	t.Setenv("FGT_CLI_SERVER", "https://env.example")

	// Env overrides profile.
	s, err := Resolve(f, Overrides{Profile: "lab"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Server != "https://env.example" {
		t.Errorf("env should win over profile, got %q", s.Server)
	}

	// Flag overrides env.
	s, err = Resolve(f, Overrides{Profile: "lab", Server: "https://flag.example"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Server != "https://flag.example" {
		t.Errorf("flag should win over env, got %q", s.Server)
	}
}

func TestValidate_TokenRequiresSecret(t *testing.T) {
	s := &Settings{Server: "https://x", AuthType: "token"}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for missing token secret")
	}
	s.Secret = "abc"
	if err := s.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidate_Session(t *testing.T) {
	// A complete session config validates.
	ok := &Settings{Server: "https://x", AuthType: "session", User: "admin", Secret: "pw"}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid session config rejected: %v", err)
	}
	// Missing username / password each fail.
	if err := (&Settings{Server: "https://x", AuthType: "session", Secret: "pw"}).Validate(); err == nil {
		t.Error("session auth without a username should fail")
	}
	if err := (&Settings{Server: "https://x", AuthType: "session", User: "admin"}).Validate(); err == nil {
		t.Error("session auth without a password should fail")
	}
}
