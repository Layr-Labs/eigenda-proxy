# Memstore Backend

The Memstore backend is a simple in-memory key-value store that is meant to replace a real EigenDA backend (talking to the disperser) for testing and development purposes. It is **never** recommended for production use.

## Usage

```bash
./bin/eigenda-proxy --memstore.enabled
```

## Configuration

See [memconfig/config.go](./memconfig/config.go) for the configuration options.
These can all be set via their respective flags or environment variables. Run `./bin/eigenda-proxy --help | grep memstore` to see these.

## Config REST API

The Memstore backend also provides a REST API for changing the configuration at runtime. This is useful for testing different configurations without restarting the proxy.

The API consists of GET and PATCH methods on the `/memstore/config` resource.

### Get the current configuration

```bash
$ curl http://localhost:3100/memstore/config | jq
{
  "MaxBlobSizeBytes": 16777216,
  "BlobExpiration": "25m0s",
  "PutLatency": "0s",
  "GetLatency": "0s",
  "PutReturnsFailoverError": false,
  "GetReturnsInstructedStatusCode": {}
}
```

### Set a configuration option

The PATCH request allows to patch the configuration. This allows only sending a subset of the configuration options. The other fields will be left intact.

```bash
$ curl -X PATCH http://localhost:3100/memstore/config -d '{"PutReturnsFailoverError": true}'
{"MaxBlobSizeBytes":2048,"BlobExpiration":"45m0s","PutLatency":"0s","GetLatency":"0s","PutReturnsFailoverError":true,"GetReturnsInstructedStatusCode":{}}
```

One can of course still build a jq pipe to produce the same result (although still using PATCH instead of PUT since that is the only method available):
```bash
$ curl http://localhost:3100/memstore/config | \
  jq '.PutLatency = "5s" | .GetLatency = "2s"' | \
  curl -X PATCH http://localhost:3100/memstore/config -d @-
```

### Instructed Status Code Return
The instructed status code return allows users to set a desired returned status code for some payloads the user is about to write. The next time when a user requests to get the payloads with keys, the proxy returns the result corresponding to the status code set earlier. The status code is sticky, and
can affect all subsequent writes. By default, the memstore isinitialized without the instructed status code return.

```bash
 curl -X PATCH http://localhost:3100/memstore/config -d '{"GetReturnsInstructedStatusCode": {"GetReturnsStatusCode": 3, "IsActivated": true }}'
 {"MaxBlobSizeBytes":2048,"BlobExpiration":"45m0s","PutLatency":"0s","GetLatency":"0s","PutReturnsFailoverError":false,"GetReturnsInstructedStatusCode":{"GetReturnsStatusCode":3,"IsActivated":true}}
```

A user can only activate the instructed status code via http PATCH method above. A user can switch to other status code from an existing activated status code
by sending a new `PATCH` request, the GET for subsequent writes contains the new status code.

A very important invariant is that no key can ever be overwritten. This is important for all rollup use cases.
### Golang client
A simple HTTP client implementation lives in `/clients/memconfig_client/` and can be imported for manipulating the config using more structured types.