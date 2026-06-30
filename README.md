# node3.cloud

Managed Rubix node platform — API access, webhooks, non-custodial signing, and hosted smart contracts. Free to start.

**[node3.cloud](https://node3.cloud)**

---

## What it is

Running a Rubix node means managing IPFS, P2P networking, port forwarding, and a database before writing a single line of your app. node3.cloud does all of that so you don't have to.

You get:
- A managed Rubix node with a REST API authenticated by API key
- Webhooks that fire HTTP callbacks when transactions settle on-chain
- Non-custodial signing — your private key stays in your browser, derived from your BIP-39 mnemonic
- Hosted smart contracts — upload WASM, get execution logs and state history per run

## Architecture

```
nginx (TLS, static portal)
  └── platform (Go, :8080)
        ├── postgres (platformdb — accounts, sessions, keys, webhooks, contracts)
        └── node0 (rubixgoplatform, :20000)
              └── postgres (rubixdb — fullnode_transactions, tokens, DIDs)
```

Four Docker Compose services on a private bridge. Only nginx is internet-facing (ports 80/443). Rubix P2P ports (21000, 22000, 4002) are exposed directly for network participation.

## Stack

- **Platform**: Go, chi router, pgx v5, wazero (WASM runtime, CGO-free)
- **Portal**: SvelteKit (Svelte 5 runes), Tailwind CSS v4, static SPA via `adapter-static`
- **Crypto**: BIP-39/BIP-44 at `m/44'/9999'/0'/0/0` in-browser, `@noble/secp256k1`, PBKDF2 + AES-GCM for encrypted IndexedDB storage
- **Payments**: Lemon Squeezy subscriptions, webhook-driven tier management

## Development

```sh
# Frontend (portal)
npm install
npm run dev

# Platform (requires Go 1.22+ and Postgres)
cd platform
go run ./cmd/server
```

For local dev, set `DEV_CORS_ORIGIN=http://localhost:5173` in your environment so the platform accepts requests from the Vite dev server.

See [DEPLOYMENT.md](DEPLOYMENT.md) for production deployment on a fresh VM.

## Plans

| | Free | Pro |
|---|---|---|
| Node | Shared | Dedicated |
| Requests | 10,000 / month | Higher limits |
| API keys | 1 | Multiple |
| Webhooks | 3 subscriptions | Unlimited |
| Hosted contracts | 1 | Multiple |

## License

MIT

---

Sponsored by [Fexr](https://getfexr.com)
