package bolt

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var handshake = [...]byte{
	// magic preamble
	0x60, 0x60, 0xB0, 0x17,

	// supported versions
	0x00, 0x00, 0x00, 0x01,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
}

type version [4]byte

var noSupportedVersions = version{0x00, 0x00, 0x00, 0x00}
var version1_0 = version{0x00, 0x00, 0x00, 0x01}

const (
	// Version is the current version of this driver
	Version = "3.3"

	// ClientID is the id of this client
	ClientID = "SermoDigitalBolt/" + Version

	DefaultPort = "7687"      // default port for regular socket connections
	DefaultHost = "localhost" // default host for regular socket connections
	Scheme      = "bolt://"   // Bolt protocol's URI scheme
)

// Open calls DialOpen with the default dialer.
func Open(name string) (driver.Conn, error) {
	switch os.Getenv(TLSEnv) {
	case "1", "true":
		drv, err := TLSDialer("", "", "", false)
		if err != nil {
			return nil, err
		}
		return DialOpen(drv, name)
	default:
		return DialOpen(&dialer{}, name)
	}
}

// DialOpen opens a driver.Conn with the given Dialer and network configuration.
func DialOpen(d Dialer, name string) (driver.Conn, error) {
	nc, v, err := open(d, name)
	if err != nil {
		return nil, err
	}
	conn, err := newConn(nc, v)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// parseTimeout returns the timeout in seconds.
func parseTimeout(tos string) (time.Duration, error) {
	if tos == "" || tos == "0" {
		return 0, nil
	}
	timeout, err := strconv.ParseInt(tos, 0, 0)
	if err != nil {
		return 0, err
	}
	return time.Duration(timeout) * time.Second, nil
}

func open(d Dialer, name string) (net.Conn, values, error) {
	// Default Neo4j configuration information.
	v := values{"host": DefaultHost, "port": DefaultPort}

	// Parse environment variables if applicable. These will be overwritten by
	// the URI configuration if it exists.
	v.merge(parseEnv())

	// Parse our values from the URL if applicable.
	if strings.HasPrefix(name, Scheme) {
		err := parseURL(v, name)
		if err != nil {
			return nil, nil, err
		}
	}

	conn, err := dial(d, v)
	if err != nil {
		return nil, nil, err
	}
	return conn, v, nil
}

func dial(d Dialer, v values) (net.Conn, error) {
	addr := net.JoinHostPort(v.get("host"), v.get("port"))
	timeout, err := parseTimeout(v.get("dial_timeout"))
	if err != nil {
		return nil, err
	}
	if timeout != 0 {
		return d.DialTimeout("tcp", addr, timeout)
	}
	return d.Dial("tcp", addr)
}

const (
	// HostEnv is the environment variable read to gather the host information.
	HostEnv = "BOLT_DRIVER_HOST"

	// PortEnv is the environment variable read to gather the port information.
	PortEnv = "BOLT_DRIVER_PORT"

	// UserEnv is the environment variable read to gather the username.
	UserEnv = "BOLT_DRIVER_USER"

	// PassEnv is the environment variable read to gather the password.
	PassEnv = "BOLT_DRIVER_PASS"

	// TLSEnv is the environment variable read to determine whether the Open
	// and OpenNeo methods should attempt to connect with TLS.
	TLSEnv = "BOLT_DRIVER_TLS"

	// TLSNoVerifyEnv is the environment variable read to determine whether
	// the TLS certificate's verification should be skipped.
	TLSNoVerifyEnv = "BOLT_DRIVER_NO_VERIFY"

	// TLSCACertFileEnv is the environment variable read that should contain
	// the CA certificate's path.
	TLSCACertFileEnv = "BOLT_DRIVER_TLS_CA_CERT_FILE"

	// TLSCertFileEnv is the environment variable read that should contain the
	// public key path.
	TLSCertFileEnv = "BOLT_TLS_CERT_FILE"

	// TLSKeyFileEnv is the environment variable read that should contain the
	// private key path.
	TLSKeyFileEnv = "BOLT_TLS_KEY_FILE"
)

func parseEnv() values {
	v := make(values)
	for _, val := range os.Environ() {
		parts := strings.SplitN(val, "=", 2)
		switch p := parts[1]; parts[0] {
		case HostEnv:
			v.set("host", p)
		case PortEnv:
			v.set("port", p)
		case UserEnv:
			v.set("user", p)
		case PassEnv:
			v.set("password", p)
		}
	}
	return v
}

func parseURL(v values, name string) error {
	url, err := url.Parse(name)
	if err != nil {
		return err
	}
	host, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		return err
	}
	v.set("host", host)
	v.set("port", port)
	if url.User != nil {
		pw, ok := url.User.Password()
		if !ok {
			return errors.New("if username is provided a password is required")
		}
		v.set("password", pw)
		v.set("username", url.User.Username())
	}
	m := url.Query()
	set := func(key string) {
		v.set(key, m.Get(key))
	}
	set("timeout")
	set("tls")
	set("tls_ca_cert_file")
	set("tls_cert_file")
	set("tls_key_file")
	set("tls_no_verify")
	return nil
}

// Dialer is a generic interface for types that can dial network addresses.
type Dialer interface {
	// Dial connects to the address on the named network.
	Dial(network, address string) (net.Conn, error)

	// DialTimeout acts like Dial but takes a timeout. The timeout should
	// included name resolution, if required.
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
}

// TLSDialer returns a Dialer that is compatible with Neo4j. It can be passed
// to DialOpen and DialOpenNeo. It reads configuration information from
// environment variables, although the function parameters take precedence.
// noVerify will only be read from an environment variable if noVerify is false.
func TLSDialer(caFile, certFile, keyFile string, noVerify bool) (Dialer, error) {
	if caFile == "" {
		caFile = os.Getenv(TLSCACertFileEnv)
	}
	if certFile == "" {
		certFile = os.Getenv(TLSCertFileEnv)
	}
	if keyFile == "" {
		keyFile = os.Getenv(TLSKeyFileEnv)
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS12}

	if caFile != "" {
		cert, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(cert) {
			return nil, errors.New("could not append CA certificate")
		}
		cfg.RootCAs = pool
	}

	if certFile != "" {
		if keyFile == "" {
			return nil, errors.New("cert file requires a key file")
		}
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	if !noVerify {
		noVerify = os.Getenv(TLSNoVerifyEnv) == "1"
	}
	cfg.InsecureSkipVerify = noVerify
	return &dialer{cfg: cfg}, nil
}

// dialer is the default Dialer. It'll use TLS if its cfg member is set,
// typically through calling TLSDialer.
type dialer struct {
	cfg *tls.Config
}

// Dial implements Dialer.
func (d *dialer) Dial(network, addr string) (net.Conn, error) {
	if d.cfg != nil {
		return tls.Dial(network, addr, d.cfg)
	}
	return net.Dial(network, addr)
}

// Dial implements Dialer.
func (d *dialer) DialTimeout(network, addr string, timeout time.Duration) (net.Conn, error) {
	if d.cfg != nil {
		return tls.DialWithDialer(&net.Dialer{Timeout: timeout}, network, addr, d.cfg)
	}
	return net.DialTimeout(network, addr, timeout)
}

type drv struct{}

// Open opens a new Bolt connection to the Neo4J database
func (d *drv) Open(name string) (driver.Conn, error) {
	return Open(name)
}

type values map[string]string

func (v values) set(k, vv string) {
	v[k] = vv
}

func (v values) get(k string) string {
	return v[k]
}

// merge adds v2 to v, overwriting any new entries.
func (v values) merge(v2 values) {
	for k, vv := range v2 {
		v.set(k, vv)
	}
}

const (
	DefaultDriver   = "bolt"
	RecordingDriver = "bolt-recorder"
)

func init() {
	sql.Register(DefaultDriver, &drv{})
	sql.Register(RecordingDriver, &Recorder{})
}
