package db

import (
	"database/sql"
	"errors"
	"testing"
)

func TestSSOProviderLifecycle(t *testing.T) {
	RunTestWithBothDBs(t, "lifecycle", func(t *testing.T, store *Store) {
		first := SSOProvider{ID: "idp-first", Template: "google", Name: "Google", IssuerURL: "https://accounts.google.com", ClientID: "first", ClientSecret: "secret", AutoProvision: true, Enabled: true, SortOrder: 0}
		second := SSOProvider{ID: "idp-second", Template: "oidc", Name: "Company", IssuerURL: "https://id.example.com", ClientID: "second", ClientSecret: "secret", SortOrder: 1}
		if err := store.CreateSSOProvider(first); err != nil {
			t.Fatalf("create first provider: %v", err)
		}
		if err := store.CreateSSOProvider(second); err != nil {
			t.Fatalf("create second provider: %v", err)
		}

		providers, err := store.ListSSOProviders()
		if err != nil || len(providers) != 2 || providers[0].ID != first.ID {
			t.Fatalf("unexpected providers: %#v, %v", providers, err)
		}
		second.Name = "Work SSO"
		second.SortOrder = 1
		if err := store.UpdateSSOProvider(second); err != nil {
			t.Fatalf("update provider: %v", err)
		}
		stored, err := store.GetSSOProvider(second.ID)
		if err != nil || stored.Name != "Work SSO" {
			t.Fatalf("unexpected updated provider: %#v, %v", stored, err)
		}

		if err := store.ReorderSSOProviders([]string{second.ID, first.ID}); err != nil {
			t.Fatalf("reorder providers: %v", err)
		}
		providers, _ = store.ListSSOProviders()
		if providers[0].ID != second.ID || providers[1].ID != first.ID {
			t.Fatalf("unexpected order: %#v", providers)
		}
		order, err := store.NextSSOProviderSortOrder()
		if err != nil || order != 2 {
			t.Fatalf("unexpected next sort order: %d, %v", order, err)
		}
		if err := store.DeleteSSOProvider(first.ID); err != nil {
			t.Fatalf("delete provider: %v", err)
		}
		if err := store.DeleteSSOProvider(first.ID); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("expected sql.ErrNoRows, got %v", err)
		}
	})
}

func TestDeleteSSOProviderWithLinkedUser(t *testing.T) {
	RunTestWithBothDBs(t, "linked-user", func(t *testing.T, store *Store) {
		provider := SSOProvider{ID: "idp-linked", Template: "oidc", Name: "Company", IssuerURL: "https://id.example.com", ClientID: "client", ClientSecret: "secret"}
		if err := store.CreateSSOProvider(provider); err != nil {
			t.Fatalf("create provider: %v", err)
		}
		if _, err := store.FindOrCreateSSOUser(provider.ID, "subject", "person@example.com", "Person", "", true); err != nil {
			t.Fatalf("create linked user: %v", err)
		}
		if err := store.DeleteSSOProvider(provider.ID); !errors.Is(err, ErrSSOProviderInUse) {
			t.Fatalf("expected ErrSSOProviderInUse, got %v", err)
		}
	})
}

func TestResetRecreatesSSOProvidersTable(t *testing.T) {
	RunTestWithBothDBs(t, "reset", func(t *testing.T, store *Store) {
		provider := SSOProvider{ID: "idp-reset", Template: "oidc", Name: "Reset", IssuerURL: "https://id.example.com", ClientID: "client", ClientSecret: "secret"}
		if err := store.CreateSSOProvider(provider); err != nil {
			t.Fatal(err)
		}
		if err := store.Reset(); err != nil {
			t.Fatalf("reset store: %v", err)
		}
		providers, err := store.ListSSOProviders()
		if err != nil {
			t.Fatalf("list providers after reset: %v", err)
		}
		if len(providers) != 0 {
			t.Fatalf("expected no providers after reset, got %#v", providers)
		}
	})
}

func TestSSOLoginDoesNotTakeOverPasswordAccount(t *testing.T) {
	RunTestWithBothDBs(t, "password-account", func(t *testing.T, store *Store) {
		if err := store.CreateUser("person", "Password123!", "UTC", "editor"); err != nil {
			t.Fatal(err)
		}
		user, err := store.Authenticate("person", "Password123!")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.db.Exec(store.rebind("UPDATE users SET email = ? WHERE id = ?"), "person@example.com", user.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.FindOrCreateSSOUser("idp-company", "subject", "person@example.com", "Person", "", true); !errors.Is(err, ErrAccountLinkingNeed) {
			t.Fatalf("expected ErrAccountLinkingNeed, got %v", err)
		}
	})
}

func TestSSOOnlyAccountCanMoveToAnotherProvider(t *testing.T) {
	RunTestWithBothDBs(t, "sso-relink", func(t *testing.T, store *Store) {
		original, err := store.FindOrCreateSSOUser("idp-first", "first-subject", "person@example.com", "Person", "", true)
		if err != nil {
			t.Fatal(err)
		}
		relinked, err := store.FindOrCreateSSOUser("idp-second", "second-subject", "person@example.com", "Person", "", true)
		if err != nil {
			t.Fatal(err)
		}
		if relinked.ID != original.ID || relinked.SSOProvider != "idp-second" || relinked.Role != original.Role {
			t.Fatalf("unexpected relinked user: %#v", relinked)
		}
	})
}
