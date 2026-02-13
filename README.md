# node3.cloud

Your personal web3 dev workbench — tools, history, and AI help, all saved in your browser. You don't need cloud for everything, just like the name.

**[node3.cloud](https://node3.cloud)** is a fully offline-capable, browser-native toolkit for Ethereum and EVM developers. No accounts, no servers, no tracking. Everything runs client-side and persists in IndexedDB.

## Why

Every web3 dev has a dozen browser tabs open — one for converting wei, another for hashing, another for decoding calldata. node3.cloud puts them all in one place that loads instantly, works offline, and never phones home.

## Tools

### Visualize
- **Tx Flow Mapper** — Interactive canvas for mapping transaction flows between addresses. Add nodes, connect them with edges, label with amounts/hashes, pan & zoom. Persists across sessions.

### Convert
- **Unit Converter** — Wei / Gwei / ETH conversion with full precision
- **Hex Converter** — Hex / Decimal / Binary interconversion
- **Base64 Codec** — Base64 / UTF-8 / Hex encoding and decoding
- **Epoch Converter** — Unix timestamp to human-readable date and back
- **Checksum Address** — EIP-55 mixed-case checksum validation and formatting

### Generate
- **Keccak256** — Hash any input string
- **Function Selector** — Compute 4-byte selectors from function signatures

### Decode
- **ABI Encoder** — Encode and decode ABI-encoded calldata

## Contributing a Tool

This is where you come in. If you've ever built a one-off script or helper for your web3 workflow, it probably belongs here.

**Tools we'd love to see contributed:**

- **RLP Encoder/Decoder** — Encode and decode RLP-serialized data
- **Merkle Tree Builder** — Construct and verify Merkle proofs
- **Storage Slot Calculator** — Compute Solidity storage slot positions for mappings and arrays
- **Event Log Decoder** — Paste raw log data, decode with ABI
- **Signature Recovery** — Recover signer address from a signed message
- **CREATE2 Address Calculator** — Predict contract deployment addresses
- **Gas Estimator** — Estimate gas costs for common operations at current prices
- **Bytecode Disassembler** — Disassemble EVM bytecode into opcodes
- **ENS Namehash** — Compute namehash for ENS domains
- **Chain ID Lookup** — Search chain IDs, RPCs, explorers across EVM networks
- **HD Wallet Derivation Path Explorer** — Visualize BIP-44 derivation paths
- **Transaction Builder** — Construct raw unsigned transactions
- **Calldata Decoder** — Decode raw calldata against known function signatures

Or bring your own idea. If it helps your daily workflow, it'll help others.

### How to Add a Tool

This project was built with AI (Claude) and we actively encourage you to do the same. Point your AI coding tool at this repo, describe the tool you want, and ship it. The codebase is structured to make that easy.

1. Fork the repo
2. Create your component in `src/lib/tools/YourTool.svelte` — follow any existing tool as a template
3. Register it in `src/lib/stores.svelte.ts` — add the ID to `ToolId`, add a `ToolDef` entry
4. Add the conditional render in `src/routes/+page.svelte`
5. Open a PR

**Constraints:**
- Must work entirely in the browser. No external API calls for core functionality.
- Use the existing design tokens (`text-text`, `bg-bg-surface`, `border-border`, `text-accent`, etc.)
- Keep dependencies minimal. Prefer `@noble/*` and `ethers` which are already in the project.
- History integration via `addHistoryEntry()` from `$lib/db` is encouraged but optional.
- AI-generated PRs are welcome. Just make sure it builds and the tool works.

## Tech Stack

- [SvelteKit](https://svelte.dev) with Svelte 5 runes
- [Tailwind CSS v4](https://tailwindcss.com)
- [Dexie](https://dexie.org) (IndexedDB wrapper)
- [ethers.js](https://docs.ethers.org)
- [@noble/hashes](https://github.com/paulmillr/noble-hashes)
- Static build via `@sveltejs/adapter-static`, deployed to GitHub Pages

## Development

```sh
npm install
npm run dev
```

Build for production:

```sh
npm run build
npm run preview
```

## License

MIT

---

Sponsored by [Fexr](https://getfexr.com)
