// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"context"
	"crypto/tls"
	"database/sql"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MagicalWig34653/db-manager/internal/core"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

var dnsHost = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?$`)

func canonicalHost(host string) string { return strings.TrimSuffix(strings.ToLower(host), ".") }
func validHost(host string) bool {
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Zone() == ""
	}
	return dnsHost.MatchString(host) && !strings.Contains(host, "..")
}
func permittedIP(ip netip.Addr) bool {
	return ip.IsValid() && ip.Zone() == "" && !ip.IsUnspecified() && !ip.IsMulticast() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}
func (e *Engine) network(ctx context.Context, c core.Connection) (*sql.DB, error) {
	host := canonicalHost(c.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	if !validHost(host) || !e.hosts[host] {
		return nil, public("target_not_allowed", "Datenbankziel ist administrativ nicht freigegeben.")
	}
	var ips []netip.Addr
	if host == "localhost" {
		ips = []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.IPv6Loopback()}
	} else if ip, err := netip.ParseAddr(host); err == nil {
		ips = []netip.Addr{ip}
	} else {
		var err error
		ips, err = net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, public("connection_failed", "Datenbankziel konnte nicht aufgelöst werden.")
		}
	}
	if len(ips) == 0 {
		return nil, public("connection_failed", "Datenbankziel konnte nicht aufgelöst werden.")
	}
	for _, ip := range ips {
		if !permittedIP(ip.Unmap()) {
			return nil, public("target_not_allowed", "Datenbankziel enthält eine nicht erlaubte Netzwerkadresse.")
		}
	}
	port := c.Port
	if port == 0 {
		port = 3306
		if c.Driver == "postgres" {
			port = 5432
		}
	}
	// The dialer never performs a second DNS lookup and never accepts driver fallback hosts.
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		var last error
		d := net.Dialer{Timeout: 5 * time.Second}
		for _, ip := range ips {
			conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), strconv.Itoa(port)))
			if err == nil {
				return conn, nil
			}
			last = err
			if ctx.Err() != nil {
				break
			}
		}
		return nil, last
	}
	var tlsConfig *tls.Config
	if c.TLSMode != "disable" {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host, RootCAs: e.roots}
	}
	if c.Driver == "mysql" {
		cfg := mysql.NewConfig()
		cfg.Net = "tcp"
		cfg.Addr = net.JoinHostPort(host, strconv.Itoa(port))
		cfg.User = c.Username
		cfg.Passwd = c.Password
		cfg.DBName = c.Database
		cfg.TLS = tlsConfig
		cfg.DialFunc = dial
		cfg.Timeout = 5 * time.Second
		cfg.MultiStatements = false
		connector, err := mysql.NewConnector(cfg)
		if err != nil {
			return nil, err
		}
		return sql.OpenDB(connector), nil
	}
	// A fixed explicit seed prevents ambient PGHOST/PGUSER/passfile credentials from
	// becoming request defaults. Every relevant connection policy is replaced below.
	cfg, err := pgx.ParseConfig("postgres://unconfigured:unconfigured@127.0.0.1:5432/postgres?sslmode=disable")
	if err != nil {
		return nil, err
	}
	cfg.Host = host
	cfg.Port = uint16(port)
	cfg.User = c.Username
	cfg.Password = c.Password
	cfg.Database = c.Database
	cfg.TLSConfig = tlsConfig
	cfg.Fallbacks = nil
	cfg.DialFunc = dial
	cfg.ConnectTimeout = 5 * time.Second
	cfg.ValidateConnect = nil
	cfg.RuntimeParams = map[string]string{"client_encoding": "UTF8"}
	cfg.LookupFunc = func(context.Context, string) ([]string, error) { return []string{ips[0].String()}, nil }
	return stdlib.OpenDB(*cfg), nil
}
