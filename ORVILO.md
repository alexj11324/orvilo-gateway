# Orvilo gateway

The gateways [Orvilo](https://github.com/alexj11324/orvilo1) runs, derived from
[lobehub/lobehub-gateway](https://github.com/lobehub/lobehub-gateway) with the
changes Orvilo needs. The Orvilo repository builds its device gateway and agent
gateway from a pinned commit of this one, for production and for local
development alike.

## What differs from upstream

Upstream base: `fed50da75b8916566f87e32fafd852a8a34e336b` (LobeHub Gateway 0.4.0).

| Commit subject | What it adds |
| --- | --- |
| `feat(device-gateway): add the Orvilo workspace protocol` | Separate personal and workspace device pools, workspace JWT purpose/subject/claim checks, workspace HTTP dispatch, API-key auth limited to `ORVILO_API_BASE_URL` |
| `feat(agent-gateway): apply the Orvilo backend-origin change` | The backend-origin change the agent gateway is deployed with |
| `refactor: rename the project to Orvilo Gateway` | Go module paths, product names in docs and release titles, and `ORVILO_API_BASE_URL` for the agent gateway (the upstream variable name is no longer read) |

`git log fed50da..master` lists every Orvilo change. Everything below that
commit is upstream history, unmodified.

The rename touches import paths in most Go files, so an upstream merge will
conflict on import lines; resolve them by keeping the Orvilo module path.

## Taking an upstream update

```bash
git remote add upstream https://github.com/lobehub/lobehub-gateway.git
git fetch upstream
git merge upstream/master
```

Then, in each of `device-gateway-go` and `agent-gateway-go`: `gofmt -l .` prints
nothing, `go vet ./...` and `go test -race ./...` pass. Update the base commit
above.

## Run locally

```bash
cd device-gateway-go
SERVICE_TOKEN=<token> JWKS_PUBLIC_KEY='<public JWKS JSON>' PORT=8787 \
  go run ./cmd/device-gateway-go
```

`JWKS_PUBLIC_KEY` is the public half of the Orvilo server's `JWKS_KEY`; point the
server at the gateway with `DEVICE_GATEWAY_URL` and
`DEVICE_GATEWAY_SERVICE_TOKEN`.

## Licence

Upstream publishes no licence file. This repository stays private.
