package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrDockerHostNotFound = errors.New("docker host not found")

// DockerHost is a reusable connection to a Docker Engine API. Certificate fields are
// deliberately omitted from JSON; handlers expose only whether credentials exist.
type DockerHost struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Endpoint   string    `json:"endpoint"`
	TLSVerify  bool      `json:"tlsVerify"`
	CACert     string    `json:"-"`
	ClientCert string    `json:"-"`
	ClientKey  string    `json:"-"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (s *Store) CreateDockerHost(h DockerHost) error {
	_, err := s.db.Exec(s.rebind("INSERT INTO docker_hosts (id, name, endpoint, tls_verify, ca_cert, client_cert, client_key, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"),
		h.ID, h.Name, h.Endpoint, h.TLSVerify, h.CACert, h.ClientCert, h.ClientKey, time.Now())
	return err
}

func (s *Store) GetDockerHosts() ([]DockerHost, error) {
	rows, err := s.db.Query("SELECT id, name, endpoint, tls_verify, ca_cert, client_cert, client_key, created_at FROM docker_hosts ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	hosts := make([]DockerHost, 0)
	for rows.Next() {
		var h DockerHost
		if err := rows.Scan(&h.ID, &h.Name, &h.Endpoint, &h.TLSVerify, &h.CACert, &h.ClientCert, &h.ClientKey, &h.CreatedAt); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (s *Store) GetDockerHost(id string) (DockerHost, error) {
	var h DockerHost
	err := s.db.QueryRow(s.rebind("SELECT id, name, endpoint, tls_verify, ca_cert, client_cert, client_key, created_at FROM docker_hosts WHERE id = ?"), id).
		Scan(&h.ID, &h.Name, &h.Endpoint, &h.TLSVerify, &h.CACert, &h.ClientCert, &h.ClientKey, &h.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DockerHost{}, ErrDockerHostNotFound
	}
	return h, err
}

func (s *Store) UpdateDockerHost(h DockerHost) error {
	res, err := s.db.Exec(s.rebind("UPDATE docker_hosts SET name = ?, endpoint = ?, tls_verify = ?, ca_cert = ?, client_cert = ?, client_key = ? WHERE id = ?"),
		h.Name, h.Endpoint, h.TLSVerify, h.CACert, h.ClientCert, h.ClientKey, h.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrDockerHostNotFound
	}
	return nil
}

func (s *Store) DeleteDockerHost(id string) error {
	monitors, err := s.GetMonitors()
	if err != nil {
		return err
	}
	for _, monitor := range monitors {
		if monitor.RequestConfig != nil && monitor.RequestConfig.DockerHostID == id {
			return fmt.Errorf("docker host is used by monitor %q", monitor.Name)
		}
	}
	res, err := s.db.Exec(s.rebind("DELETE FROM docker_hosts WHERE id = ?"), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrDockerHostNotFound
	}
	return nil
}
