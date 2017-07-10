// Package bolt implements a driver for Neo4J's Bolt Protocol.
//
// This package provides one standard driver and one "recording" driver. The
// standard driver is simply
//
//  bolt
//
// It should be used in most circumstances.
//
// The Recorder type implements driver.Driver and can be used to read from or
// create a recorded session. See Recorder's documentation for more
// information.
//
// There are three main ways to query the database, checked in the following
// order:
//
//  1.) The first argument to any Query, Exec., etc is of the type Map and
//      no other arguments are passed. This is the easiest and cleanest option.
//
//  2.) Every argument is of the type sql.NamedArg. Each argument will be
//      interpreted as a a key-value pair. If the key == "", an error will be
//      returned.
//
//  3.) An even number of arguments in key-value order, meaning the
//      even-indexed values must be of the type string.
//
// The connection URI format is:
//
//	bolt://[user[:password]]@[host][:port][?param1=value1&...]
//
// Parameters are as follows:
//
//	- dial_timeout:     Timeout for dialing a new connection in seconds.
//	- timeout:          Read and write timeout in seconds.
//	- tls:              Should the connection use TLS? 1 or 0.
//	- tls_ca_cert_file: Path to CA certificate file.
//	- tls_cert_file:    Path to certificate file.
//	- tls_key_file:     Path to key file.
//	- tls_no_verify:    Should the connection _not_ verify TLS? 1 or 0.
//
// Eenvironment variables can be used, although URI parameters will take
// precedence over envirnment variables. In the same order as above:
//
//	- BOLT_DRIVER_HOST
//	- BOLT_DRIVER_PORT
//	- BOLT_DRIVER_USER
//	- BOLT_DRIVER_PASS
//	- BOLT_DRIVER_TLS
//	- BOLT_DRIVER_TLS_CA_CERT_FILE
//	- BOLT_DRIVER_TLS_CERT_FILE
//	- BOLT_DRIVER_TLS_KEY_FILE
//	- BOLT_DRIVER_NO_VERIFY
//
package bolt
