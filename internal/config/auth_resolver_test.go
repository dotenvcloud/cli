package config

import "testing"

func TestResolveAuth_EnvAPIKeyWinsOverAccount(t *testing.T) {
	env := &EnvConfig{APIKey: "ENVKEY", Organization: "org-env", APIURL: "https://env.example"}
	acct := &Account{Name: "stored", APIURL: "https://acct.example"}

	got := ResolveAuth("", env, acct)

	if got.Source != AuthSourceEnv {
		t.Fatalf("Source = %s, want %s", got.Source, AuthSourceEnv)
	}
	if got.APIKey != "ENVKEY" {
		t.Fatalf("APIKey = %q, want ENVKEY", got.APIKey)
	}
	if got.Organization != "org-env" {
		t.Fatalf("Organization = %q, want org-env", got.Organization)
	}
	if got.APIURL != "https://env.example" {
		t.Fatalf("APIURL = %q, want https://env.example", got.APIURL)
	}
	if got.UsesAccountStore {
		t.Fatal("env-credential mode must not use the account store")
	}
}

func TestResolveAuth_FlagAPIKeyWins(t *testing.T) {
	got := ResolveAuth("FLAGKEY", &EnvConfig{}, &Account{Name: "stored"})

	if got.Source != AuthSourceEnv || got.APIKey != "FLAGKEY" {
		t.Fatalf("got %+v, want env source with FLAGKEY", got)
	}
	if got.UsesAccountStore {
		t.Fatal("flag api key must not use the account store")
	}
}

func TestResolveAuth_AccountWhenNoEnvCredential(t *testing.T) {
	acct := &Account{Name: "stored", APIURL: "https://acct.example"}

	got := ResolveAuth("", &EnvConfig{}, acct)

	if got.Source != AuthSourceAccount {
		t.Fatalf("Source = %s, want %s", got.Source, AuthSourceAccount)
	}
	if !got.UsesAccountStore {
		t.Fatal("account mode must use the account store")
	}
	if got.Label != "stored" {
		t.Fatalf("Label = %q, want stored", got.Label)
	}
	if got.Account != acct {
		t.Fatal("Account not threaded through")
	}
	if got.APIURL != "https://acct.example" {
		t.Fatalf("APIURL = %q, want account URL", got.APIURL)
	}
}

func TestResolveAuth_NoneWhenNothingAvailable(t *testing.T) {
	got := ResolveAuth("", &EnvConfig{}, nil)
	if got.Source != AuthSourceNone {
		t.Fatalf("Source = %s, want %s", got.Source, AuthSourceNone)
	}
}

func TestUsingEnvCredential(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	if UsingEnvCredential("") {
		t.Fatal("no flag and no env var should be false")
	}
	if !UsingEnvCredential("flag-value") {
		t.Fatal("flag value should be true")
	}

	t.Setenv(EnvAPIKey, "env-value")
	if !UsingEnvCredential("") {
		t.Fatal("env var present should be true")
	}
}
