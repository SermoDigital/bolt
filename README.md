# Golang Neo4J Bolt Driver
[![Build Status](https://travis-ci.org/SermoDigital/bolt.svg?branch=master)](https://travis-ci.org/SermoDigital/bolt) *Tested against Golang 1.4.3 and up*

Implements the Neo4J Bolt Protocol specification:
As of the time of writing this, the current version is v3.1.0-M02

```
go get github.com/SermoDigital/bolt
```

## Features

* Neo4j Bolt low-level binary protocol support
* Connection Pooling
* TLS support
* Compatible with sql.driver

## TODO

* Message Pipelining for high concurrency

## Usage

*_Please see [the statement tests](./stmt_test.go) or [the conn tests](./conn_test.go) for A LOT of examples of usage_*

## API

*_There is much more detailed information in [the godoc](http://godoc.org/github.com/SermoDigital/bolt)_*

This implementation attempts to follow the best practices as per the Bolt
specification, but also implements compatibility with Golang's `sql.driver` interface.

As such, these interfaces closely match the `sql.driver` interfaces, but they
also provide Neo4j Bolt specific functionality in addition to the `sql.driver` interface.

It is recommended that you use the Neo4j Bolt-specific interfaces if possible.
The implementation is more efficient and can more closely support the Neo4j Bolt feature set.

The connection URI format is:
`bolt://[user[:password]]@[host][:port][?param!=value1&...]`
Schema must be `bolt`. User and password is only necessary if you are authenticating.
Parameters are as follows:

- dial_timeout: Timeout for dialing a new connection in seconds.
- timeout: Read and write timeout in seconds.
- tls: Should the connection use TLS? 1 or 0.
- tls_ca_cert_file: Path to CA certificate file.
- tls_cert_file: Path to certificate file.
- tls_key_file: Path to key file.
- tls_no_verify: Should the connection _not_ verify TLS? 1 or 0.

Additionally, environment variables can be used, although URI parameters will
take precedence over envirnment variables. In the same order as above:

- BOLT_DRIVER_HOST
- BOLT_DRIVER_PORT
- BOLT_DRIVER_USER
- BOLT_DRIVER_PASS
- BOLT_DRIVER_TLS
- BOLT_DRIVER_TLS_CA_CERT_FILE
- BOLT_DRIVER_TLS_CERT_FILE
- BOLT_DRIVER_TLS_KEY_FILE
- BOLT_DRIVER_NO_VERIFY

Connection pooling is provided out of the box with the `OpenPool` function.
You can give it the maximum number of connections to have at a time.

## Dev Quickstart

```
# Put in git hooks
ln -s ../../scripts/pre-commit .git/hooks/pre-commit
ln -s ../../scripts/pre-push .git/hooks/pre-push

# No special build steps necessary
go build

# Testing with log info and a local bolt DB, getting coverage output
NEO4J_BOLT=bolt://localhost:7687 go test -coverprofile=./tmp/cover.out -coverpkg=./... -v -race && go tool cover -html=./tmp/cover.out

# Testing with trace output for debugging
NEO4J_BOLT=bolt://localhost:7687 go test -v -race

# Testing with running recorder to record tests for CI
NEO4J_BOLT=bolt://localhost:7687 RECORD_OUTPUT=1 go test -v -race
```

The tests are written in an integration testing style.  Most of them are in the
statement tests, but should be made more granular in the future.

In order to get CI, there's a recorder mechanism so you don't need to run Neo4j
alongside the tests in the CI server. You run the tests locally against a Neo4j
instance with the RECORD_OUTPUT=1 environment variable, it generates the
recordings in the ./recordings folder. This is necessary if the tests have
changed, or if the internals have significantly changed.  Installing the git
hooks will run the tests automatically on push. If there are updated tests,
you will need to re-run the recorder to add them and push them as well.

You need access to a running Neo4J database to develop for this project, so
you can run the tests to generate the recordings.

## TODO

* Cypher Parser to implement NumInput and pre-flight checking
* More Tests
* Benchmark Tests
