/*Package bolt implements a driver for the Neo4J Bolt Protocol.

This package provides two drivers, "bolt" and "bolt-recorder" which can both
be used in two separate fashions: through `sql.Open` or this package's `OpenPool`
function.

There are pros and cons to both.

Using this package's `OpenPool` function
provides an interface similar to `sql.DB`, but with methods tuned and built
around graph-style data instead of SQL. The primary difference is how the
methods accept their arguments: this package's `DB` type accepts a
`map[string]interface{}` instead of variadic `interface{}`. The `DB`'s pooling
code is grafted from the `sql` package with generic cases remove. Additionally,
a the interface is tweaked a bit to, for example, allow the caller to retrieve
row-based metadata.

Using `OpenPool` is advised--it's much easier and more efficient than using
`sql.Open`. However, the latter will work just fine. There are some caveats,
though. For instance, there are two primary ways to call the various `Exec`,
`Query`, etc. methods.

The first is with a `map[string]interface{}`. If the first argument has the
type `map[string]interface{}` not other arguments are allowed and will cause
the methods to return errors.

The second is passing a slice of key-value pairs, a la a denatured map. If
this option is chosen the even-numbered arguments must be have the type
`string`. Odd-numbered arguments can be anything, however it is recommended
they satisfy `driver.Value` or `driver.Valuer`.

Note that if the first convention is used or if the second convention is used
and any of the arguments do not implement either `driver.Value` or `driver.Valuer`
then they'll be marshalled into []byte per bolt's encoding protocol,
re-marshalled into their respective arguments so they can be used in the
Neo4j-specific interfaces, and re-marshalled back into []byte to be sent over
the wire.

`sql.Open` is, in effect, a thin wrapper around `OpenPool` that may or may not
require an extra serialization step in order to skirt package `sql`'s limitations.

To reiterate: using `OpenPool` is recommended, but `sql.Open` will work in most
cases.

The connection URI format is:

	bolt://[user[:password]]@[host][:port][?param1=value1&...]

Schema must always be `bolt`. User and password is only necessary if you are
authenticating.

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

*/
package bolt
