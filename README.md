# bingo
> Bingo knows everyone's name-o

Bingo is a DID <-> Handle resolution service for ATProto.

It is built using [ConnectRPC](https://connectrpc.com/) and can be accessed via GRPC, GRPC-Web, or Connect's own HTTP-based protocol.

Bingo runs a Connect server and a Directory Daemon.
- The Connect server responds to RPC requests
- The Directory Daemon discovers `did:plc` entities to track and validates `did:plc <-> handle` relationships regularly

## Running Bingo

To run Bingo using `docker compose` run:
```bash
$ make up
```

This starts up three containers to support Bingo:
- A Redis instance for lookups
- A Postgres instance to store durable data about entries and to enqueue/track validation status
- The Bingo server

Once started, you can access the Bingo service at `http://localhost:8923`

## Using Bingo

To use Bingo, you can depend on the Connect client packages like in the example in `cmd/client/main.go`.

An example raw lookup request to resolve `jaz.bsky.social` to its DID and validation status looks like:

```bash
curl --location 'http://localhost:8923/bingo.v1.BingoService/Lookup' \
--header 'Content-Type: application/json' \
--data '{
    "handle_or_did": "jaz.bsky.social"
}'
```
