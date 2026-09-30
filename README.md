# nethvoice-chat

Go programs around the per-tenant ejabberd of [ns8-nethvoice](https://github.com/nethesis/ns8-nethvoice).

- `chat-extauth`: ejabberd external auth; the password is the CTI JWT (Chat permission required).
- `chat-gateway`: XEP-0357 push (Web Push, Acrobits PNM) and the Acrobits `fetch_messages`, `send_message`, `push_token_report` services.

Configuration comes from `CHAT_*` environment variables set by the module units (see `cmd/chat-gateway/main.go`).

## Build

```sh
go vet ./... && go test ./...
CGO_ENABLED=0 go build -o dist/ ./cmd/...
podman build -t ghcr.io/nethesis/nethvoice-chat-gateway:dev -f cmd/chat-gateway/Containerfile dist
```

A `v*` tag publishes the extauth binary (with sha256) and the gateway image.
