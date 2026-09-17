package db

import (
	"database/sql"
	"errors"
	"time"
)

type SSOProvider struct {
	ID             string    `json:"id"`
	Template       string    `json:"template"`
	Name           string    `json:"name"`
	IssuerURL      string    `json:"issuerUrl"`
	ClientID       string    `json:"clientId"`
	ClientSecret   string    `json:"-"`
	AllowedDomains string    `json:"allowedDomains"`
	AutoProvision  bool      `json:"autoProvision"`
	Enabled        bool      `json:"enabled"`
	SortOrder      int       `json:"sortOrder"`
	CreatedAt      time.Time `json:"createdAt"`
}

var ErrSSOProviderInUse = errors.New("SSO provider has linked users")

func (s *Store) ListSSOProviders() ([]SSOProvider, error) {
	rows, err := s.db.Query(`SELECT id, template, name, issuer_url, client_id, client_secret, allowed_domains, auto_provision, enabled, sort_order, created_at FROM sso_providers ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var providers []SSOProvider
	for rows.Next() {
		var provider SSOProvider
		if err := rows.Scan(&provider.ID, &provider.Template, &provider.Name, &provider.IssuerURL, &provider.ClientID, &provider.ClientSecret, &provider.AllowedDomains, &provider.AutoProvision, &provider.Enabled, &provider.SortOrder, &provider.CreatedAt); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func (s *Store) GetSSOProvider(id string) (*SSOProvider, error) {
	var provider SSOProvider
	err := s.db.QueryRow(s.rebind(`SELECT id, template, name, issuer_url, client_id, client_secret, allowed_domains, auto_provision, enabled, sort_order, created_at FROM sso_providers WHERE id = ?`), id).Scan(&provider.ID, &provider.Template, &provider.Name, &provider.IssuerURL, &provider.ClientID, &provider.ClientSecret, &provider.AllowedDomains, &provider.AutoProvision, &provider.Enabled, &provider.SortOrder, &provider.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func (s *Store) CreateSSOProvider(provider SSOProvider) error {
	_, err := s.db.Exec(s.rebind(`INSERT INTO sso_providers (id, template, name, issuer_url, client_id, client_secret, allowed_domains, auto_provision, enabled, sort_order, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		provider.ID, provider.Template, provider.Name, provider.IssuerURL, provider.ClientID, provider.ClientSecret, provider.AllowedDomains, provider.AutoProvision, provider.Enabled, provider.SortOrder, time.Now().UTC())
	return err
}

func (s *Store) NextSSOProviderSortOrder() (int, error) {
	var order int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM sso_providers`).Scan(&order)
	return order, err
}

func (s *Store) UpdateSSOProvider(provider SSOProvider) error {
	result, err := s.db.Exec(s.rebind(`UPDATE sso_providers SET template = ?, name = ?, issuer_url = ?, client_id = ?, client_secret = ?, allowed_domains = ?, auto_provision = ?, enabled = ?, sort_order = ? WHERE id = ?`),
		provider.Template, provider.Name, provider.IssuerURL, provider.ClientID, provider.ClientSecret, provider.AllowedDomains, provider.AutoProvision, provider.Enabled, provider.SortOrder, provider.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) DeleteSSOProvider(id string) error {
	var users int
	if err := s.db.QueryRow(s.rebind(`SELECT COUNT(*) FROM users WHERE sso_provider = ?`), id).Scan(&users); err != nil {
		return err
	}
	if users > 0 {
		return ErrSSOProviderInUse
	}
	result, err := s.db.Exec(s.rebind(`DELETE FROM sso_providers WHERE id = ?`), id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) ReorderSSOProviders(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for order, id := range ids {
		result, err := tx.Exec(s.rebind(`UPDATE sso_providers SET sort_order = ? WHERE id = ?`), order, id)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return sql.ErrNoRows
		}
	}
	return tx.Commit()
}
