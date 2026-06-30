<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import type { TelegramUser } from '$lib/api';

	let telegramContainer: HTMLDivElement;
	let loginError = $state('');
	let loggingIn = $state(false);

	onMount(async () => {
		await auth.init();
		if (auth.isAuthenticated) {
			goto('/dashboard');
			return;
		}

		(window as any).onTelegramAuth = async (user: TelegramUser) => {
			loggingIn = true;
			loginError = '';
			try {
				await auth.loginWithTelegram(user);
				goto('/dashboard');
			} catch (e: any) {
				loginError = e.message || 'Login failed. Please try again.';
				loggingIn = false;
			}
		};

		const botUsername = import.meta.env.VITE_TELEGRAM_BOT || '';
		if (!botUsername) return;

		if (!telegramContainer) return;

		const script = document.createElement('script');
		script.src = 'https://telegram.org/js/telegram-widget.js?22';
		script.setAttribute('data-telegram-login', botUsername);
		script.setAttribute('data-size', 'large');
		script.setAttribute('data-onauth', 'onTelegramAuth(user)');
		script.async = true;
		telegramContainer.appendChild(script);
	});

	onDestroy(() => {
		delete (window as any).onTelegramAuth;
	});
</script>

<div class="min-h-screen bg-[--color-bg] text-[--color-text]">

	<!-- ── Hero ─────────────────────────────────────────────────────────── -->
	<section id="hero" class="min-h-screen flex flex-col items-center justify-center px-6 text-center">
		<p class="text-xs text-[--color-accent] mb-6 tracking-widest uppercase">Rubix Network Infrastructure</p>

		<h1 class="text-4xl md:text-6xl font-semibold text-[--color-text] leading-tight mb-6 max-w-4xl">
			The bedrock supporting Rubix's best networks, nodes, and newcomers.
		</h1>

		<p class="text-base text-[--color-text-dim] mb-10 max-w-xl leading-relaxed">
			Managed node infrastructure with real-time webhooks, non-custodial signing,
			and hosted smart contracts. Start for free — no server, no config files.
		</p>

		<div class="flex flex-col items-center gap-3">
			{#if loggingIn}
				<p class="text-sm text-[--color-text-muted]">logging in…</p>
			{:else}
				<div bind:this={telegramContainer}></div>
				{#if !import.meta.env.VITE_TELEGRAM_BOT}
					<p class="text-xs text-[--color-text-muted]">
						Set <code class="text-[--color-accent]">VITE_TELEGRAM_BOT</code> to enable login.
					</p>
				{/if}
			{/if}
			{#if loginError}
				<p class="text-xs text-[--color-red] mt-1">{loginError}</p>
			{/if}
			<a href="#features" class="text-xs text-[--color-text-muted] hover:text-[--color-text] transition-colors mt-4">
				see what's inside ↓
			</a>
		</div>
	</section>

	<!-- ── Stats bar ─────────────────────────────────────────────────────── -->
	<div class="border-y border-[--color-border]" id="features">
		<div class="max-w-3xl mx-auto grid grid-cols-3 divide-x divide-[--color-border]">
			<div class="py-8 text-center">
				<p class="text-2xl font-semibold text-[--color-text] mb-1">10,000</p>
				<p class="text-xs text-[--color-text-muted]">free requests / month</p>
			</div>
			<div class="py-8 text-center">
				<p class="text-2xl font-semibold text-[--color-text] mb-1">0</p>
				<p class="text-xs text-[--color-text-muted]">config files to manage</p>
			</div>
			<div class="py-8 text-center">
				<p class="text-2xl font-semibold text-[--color-text] mb-1">100%</p>
				<p class="text-xs text-[--color-text-muted]">non-custodial by default</p>
			</div>
		</div>
	</div>

	<!-- ── Problem statement ─────────────────────────────────────────────── -->
	<section class="max-w-2xl mx-auto px-6 py-20 text-center">
		<p class="text-lg text-[--color-text-dim] leading-relaxed">
			Running a Rubix node means managing IPFS, P2P networking, port forwarding,
			and a database — before writing a single line of your app.
		</p>
		<p class="text-lg text-[--color-text-dim] leading-relaxed mt-4">
			node3.cloud handles all of that so you don't have to.
		</p>
	</section>

	<!-- ── Feature 1 — Webhooks ──────────────────────────────────────────── -->
	<section class="max-w-5xl mx-auto px-6 py-20">
		<div class="grid md:grid-cols-2 gap-16 items-center">
			<div>
				<p class="text-xs text-[--color-accent] mb-3 tracking-widest uppercase">Webhooks</p>
				<h2 class="text-3xl font-semibold text-[--color-text] mb-5 leading-snug">
					Know when it happens.<br />Not when you check.
				</h2>
				<p class="text-sm text-[--color-text-dim] leading-relaxed mb-6">
					Subscribe to any event on the Rubix network and get an HTTP POST
					to your server the moment it settles. No polling, no missed events,
					no infrastructure to run.
				</p>
				<ul class="space-y-2.5 text-xs text-[--color-text-muted]">
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						Token transfers to or from any DID
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						Smart contract deployments and executions
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						NFT and fungible token events
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						HMAC-signed payloads via <code class="text-[--color-text-dim]">X-Rubix-Signature</code>
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						3 retry attempts with exponential backoff on failure
					</li>
				</ul>
			</div>

			<!-- Graphic: event pipeline -->
			<div class="border border-[--color-border] rounded-xl p-6 bg-[--color-bg-surface] font-mono">
				<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-5">Event delivery pipeline</p>

				<!-- Source -->
				<div class="flex items-center gap-3 mb-4">
					<div class="border border-[--color-border] rounded-lg px-3 py-2 text-[10px] text-[--color-text-muted] shrink-0">
						Rubix Network
					</div>
					<div class="flex-1 h-px bg-[--color-border]"></div>
					<div class="border border-[--color-accent] rounded-lg px-3 py-2 text-[10px] text-[--color-accent] shrink-0">
						node3.cloud
					</div>
				</div>

				<!-- Event types -->
				<div class="flex flex-wrap gap-1.5 mb-5">
					{#each ['token.received', 'token.sent', 'contract.deployed', 'contract.executed'] as evt}
						<span class="text-[10px] font-mono border border-[--color-border] rounded px-2 py-0.5 text-[--color-text-dim] bg-[--color-bg]">
							{evt}
						</span>
					{/each}
				</div>

				<!-- Delivery arrow -->
				<div class="flex items-center gap-2 mb-4">
					<div class="flex-1 h-px bg-[--color-border] border-dashed" style="border-top: 1px dashed var(--color-border-bright)"></div>
					<span class="text-[10px] text-[--color-text-muted]">HTTP POST</span>
					<div class="text-[--color-green] text-xs">→</div>
					<div class="border border-[--color-green] rounded-lg px-3 py-1.5 text-[10px] text-[--color-green]">
						Your server
					</div>
				</div>

				<!-- Payload preview -->
				<div class="border border-[--color-border] rounded-lg p-3 bg-[--color-bg]">
					<p class="text-[10px] text-[--color-text-muted] mb-2">Payload</p>
					<pre class="text-[10px] text-[--color-text-dim] leading-relaxed font-mono">{`{
  "event": "token.received",
  "data": {
    "to_did":  "bafybmi…",
    "amount":  1.5,
    "tx_id":   "…"
  }
}`}</pre>
				</div>
			</div>
		</div>
	</section>

	<!-- ── Feature 2 — Non-custodial ─────────────────────────────────────── -->
	<section class="max-w-5xl mx-auto px-6 py-20">
		<div class="grid md:grid-cols-2 gap-16 items-center">

			<!-- Graphic: key lifecycle -->
			<div class="border border-[--color-border] rounded-xl p-6 bg-[--color-bg-surface] font-mono">
				<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-6">How your key stays yours</p>
				<div class="space-y-1">

					<div class="flex items-start gap-3">
						<div class="w-6 h-6 border border-[--color-accent] rounded-full flex items-center justify-center text-[10px] text-[--color-accent] shrink-0">1</div>
						<div class="pb-4">
							<p class="text-xs text-[--color-text] mb-0.5">Generate secret phrase in browser</p>
							<p class="text-[10px] text-[--color-text-muted]">12 BIP-39 words · never transmitted · you write it down</p>
						</div>
					</div>
					<div class="ml-3 w-px h-4 bg-[--color-border]"></div>

					<div class="flex items-start gap-3">
						<div class="w-6 h-6 border border-[--color-accent] rounded-full flex items-center justify-center text-[10px] text-[--color-accent] shrink-0">2</div>
						<div class="pb-4">
							<p class="text-xs text-[--color-text] mb-0.5">Keypair derived locally</p>
							<p class="text-[10px] text-[--color-text-muted]">secp256k1 at <code>m/44'/9999'/0'/0/0</code> · stays in your browser</p>
						</div>
					</div>
					<div class="ml-3 w-px h-4 bg-[--color-border]"></div>

					<div class="flex items-start gap-3">
						<div class="w-6 h-6 border border-[--color-accent] rounded-full flex items-center justify-center text-[10px] text-[--color-accent] shrink-0">3</div>
						<div class="pb-4">
							<p class="text-xs text-[--color-text] mb-0.5">Public key only is sent to us</p>
							<p class="text-[10px] text-[--color-text-muted]">We register your DID on the node · private key never leaves you</p>
						</div>
					</div>
					<div class="ml-3 w-px h-4 bg-[--color-border]"></div>

					<div class="flex items-start gap-3">
						<div class="w-6 h-6 border border-[--color-green] rounded-full flex items-center justify-center text-[10px] text-[--color-green] shrink-0">✓</div>
						<div>
							<p class="text-xs text-[--color-text] mb-0.5">Transactions signed in-browser</p>
							<p class="text-[10px] text-[--color-text-muted]">Only the signature is transmitted · we cannot sign without you</p>
						</div>
					</div>

				</div>
			</div>

			<div>
				<p class="text-xs text-[--color-accent] mb-3 tracking-widest uppercase">Non-Custodial Signing</p>
				<h2 class="text-3xl font-semibold text-[--color-text] mb-5 leading-snug">
					Your keys.<br />Always.
				</h2>
				<p class="text-sm text-[--color-text-dim] leading-relaxed mb-6">
					Your private key is derived from your secret phrase in your browser
					and never leaves your device. node3.cloud manages your node on the
					Rubix network but cannot sign a single transaction without you.
					Most hosted node services hold your keys. We don't.
				</p>
				<ul class="space-y-2.5 text-xs text-[--color-text-muted]">
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						BIP-39 mnemonic generated entirely client-side
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						BIP-44 path matches <code class="text-[--color-text-dim]">rubixgoplatform</code> LiteDIDMode exactly
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						Encrypted with your PIN and stored in this browser only
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						Every transaction signed in-browser before submission
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						We store only the public key — auditable and verifiable
					</li>
				</ul>
			</div>
		</div>
	</section>

	<!-- ── Feature 3 — Smart contracts ───────────────────────────────────── -->
	<section class="max-w-5xl mx-auto px-6 py-20">
		<div class="grid md:grid-cols-2 gap-16 items-center">
			<div>
				<p class="text-xs text-[--color-accent] mb-3 tracking-widest uppercase">Smart Contract Hosting</p>
				<h2 class="text-3xl font-semibold text-[--color-text] mb-5 leading-snug">
					Deploy a contract.<br />Not a server.
				</h2>
				<p class="text-sm text-[--color-text-dim] leading-relaxed mb-6">
					Rubix smart contracts require a running callback server to handle executions.
					node3.cloud hosts it for you — upload your WASM binary and we handle
					generate, deploy, execution, and state management end to end.
				</p>
				<ul class="space-y-2.5 text-xs text-[--color-text-muted]">
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						Upload <code class="text-[--color-text-dim]">.wasm</code> + source — we pin it to IPFS automatically
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						Managed callback server registered on the node for you
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						Contract state persisted and versioned after every execution
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						Full execution log: input, output, state diff, timing
					</li>
					<li class="flex gap-2 items-start">
						<span class="text-[--color-accent] mt-0.5 shrink-0">—</span>
						Webhook fires on every execution — no extra setup
					</li>
				</ul>
			</div>

			<!-- Graphic: deployment pipeline -->
			<div class="border border-[--color-border] rounded-xl p-6 bg-[--color-bg-surface] font-mono">
				<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-6">Deployment lifecycle</p>
				<div>
					<div class="flex items-start gap-4">
						<div class="flex flex-col items-center shrink-0">
							<div class="w-6 h-6 border border-[--color-accent] rounded flex items-center justify-center text-[10px] font-mono text-[--color-accent]">1</div>
							<div class="w-px flex-1 bg-[--color-border] my-1" style="height: 2rem"></div>
						</div>
						<div class="pb-4">
							<p class="text-xs text-[--color-text]">Upload</p>
							<p class="text-[10px] text-[--color-text-muted]">.wasm + .rs → pinned to IPFS, contract token ID assigned</p>
						</div>
					</div>
					<div class="flex items-start gap-4">
						<div class="flex flex-col items-center shrink-0">
							<div class="w-6 h-6 border border-[--color-accent] rounded flex items-center justify-center text-[10px] font-mono text-[--color-accent]">2</div>
							<div class="w-px flex-1 bg-[--color-border] my-1" style="height: 2rem"></div>
						</div>
						<div class="pb-4">
							<p class="text-xs text-[--color-text]">Deploy</p>
							<p class="text-[10px] text-[--color-text-muted]">Consensus across Rubix quorum nodes, genesis block written</p>
						</div>
					</div>
					<div class="flex items-start gap-4">
						<div class="flex flex-col items-center shrink-0">
							<div class="w-6 h-6 border border-[--color-accent] rounded flex items-center justify-center text-[10px] font-mono text-[--color-accent]">3</div>
							<div class="w-px flex-1 bg-[--color-border] my-1" style="height: 2rem"></div>
						</div>
						<div class="pb-4">
							<p class="text-xs text-[--color-text]">Execute</p>
							<p class="text-[10px] text-[--color-text-muted]">Node callback hits our server, WASM runs in-process, state saved</p>
						</div>
					</div>
					<div class="flex items-start gap-4">
						<div class="flex flex-col items-center shrink-0">
							<div class="w-6 h-6 border border-[--color-accent] rounded flex items-center justify-center text-[10px] font-mono text-[--color-accent]">4</div>
							<div class="w-px flex-1 bg-[--color-border] my-1" style="height: 2rem"></div>
						</div>
						<div class="pb-4">
							<p class="text-xs text-[--color-text]">Observe</p>
							<p class="text-[10px] text-[--color-text-muted]">Execution log, input/output diff, current state in dashboard</p>
						</div>
					</div>
					<div class="flex items-start gap-4">
						<div class="shrink-0">
							<div class="w-6 h-6 border border-[--color-green] rounded flex items-center justify-center text-[10px] text-[--color-green]">✓</div>
						</div>
						<div>
							<p class="text-xs text-[--color-text]">Webhook fires</p>
							<p class="text-[10px] text-[--color-text-muted]">contract.executed event delivered to your callback URL</p>
						</div>
					</div>
				</div>
			</div>
		</div>
	</section>

	<!-- ── Pricing ────────────────────────────────────────────────────────── -->
	<section class="max-w-4xl mx-auto px-6 py-20">
		<div class="text-center mb-12">
			<h2 class="text-2xl font-semibold text-[--color-text] mb-3">Simple pricing.</h2>
			<p class="text-sm text-[--color-text-muted]">No credit card for the free tier. Cancel paid anytime.</p>
		</div>

		<div class="grid md:grid-cols-3 gap-6">

			<!-- Free -->
			<div class="border border-[--color-border] rounded-xl p-7 flex flex-col">
				<div class="mb-7">
					<p class="text-xs text-[--color-text-muted] uppercase tracking-widest mb-2">Free</p>
					<p class="text-4xl font-semibold text-[--color-text] mb-1">$0</p>
					<p class="text-xs text-[--color-text-muted]">per month · always</p>
				</div>

				<div class="space-y-5 flex-1">
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Infrastructure</p>
						<ul class="space-y-1.5 text-xs text-[--color-text-dim]">
							<li>1 DID on a shared node</li>
							<li>Shared IPFS peer identity</li>
							<li>Full Rubix protocol API</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">API Access</p>
						<ul class="space-y-1.5 text-xs text-[--color-text-dim]">
							<li>10,000 requests / month</li>
							<li>1 API key</li>
							<li>429 + Retry-After at limit</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Webhooks</p>
						<ul class="space-y-1.5 text-xs text-[--color-text-dim]">
							<li>3 active subscriptions</li>
							<li>All 4 event types</li>
							<li>HMAC-signed payloads</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Smart Contracts</p>
						<ul class="space-y-1.5 text-xs text-[--color-text-dim]">
							<li>1 hosted WASM contract</li>
							<li>Full execution log</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Signing</p>
						<ul class="space-y-1.5 text-xs text-[--color-text-dim]">
							<li>Non-custodial · your key always</li>
						</ul>
					</div>
				</div>

				<div class="mt-7">
					<a
						href="#hero"
						onclick={(e) => { e.preventDefault(); document.getElementById('hero')?.scrollIntoView({ behavior: 'smooth' }); }}
						class="block text-center text-xs py-3 px-4 border border-[--color-accent] text-[--color-accent] rounded-lg hover:bg-[--color-accent-dim] transition-colors"
					>
						get started free →
					</a>
					<p class="text-[10px] text-[--color-text-muted] text-center mt-3">No credit card · just Telegram</p>
				</div>
			</div>

			<!-- Pro -->
			<div class="border border-[--color-accent] rounded-xl p-7 bg-[--color-bg-surface] flex flex-col">
				<div class="mb-7">
					<p class="text-xs text-[--color-accent] uppercase tracking-widest mb-2">Pro</p>
					<p class="text-4xl font-semibold text-[--color-text] mb-1">$30</p>
					<p class="text-xs text-[--color-text-muted]">per month · billed via Lemon Squeezy</p>
				</div>

				<div class="space-y-5 flex-1">
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Infrastructure</p>
						<ul class="space-y-1.5 text-xs">
							<li class="text-[--color-text]">Dedicated Rubix node</li>
							<li class="text-[--color-text]">Own IPFS peer ID</li>
							<li class="text-[--color-text]">Own P2P identity on the network</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">API Access</p>
						<ul class="space-y-1.5 text-xs">
							<li class="text-[--color-text]">500,000 requests / month</li>
							<li class="text-[--color-text-dim]">Multiple API keys</li>
							<li class="text-[--color-text-dim]">429 + Retry-After at limit</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Webhooks</p>
						<ul class="space-y-1.5 text-xs">
							<li class="text-[--color-text]">Unlimited subscriptions</li>
							<li class="text-[--color-text-dim]">All 4 event types</li>
							<li class="text-[--color-text-dim]">HMAC-signed payloads</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Smart Contracts</p>
						<ul class="space-y-1.5 text-xs">
							<li class="text-[--color-text]">Unlimited hosted contracts</li>
							<li class="text-[--color-text-dim]">Full execution log</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Signing</p>
						<ul class="space-y-1.5 text-xs text-[--color-text-dim]">
							<li>Non-custodial · your key always</li>
						</ul>
					</div>
				</div>

				<div class="mt-7">
					<a
						href="#hero"
						onclick={(e) => { e.preventDefault(); document.getElementById('hero')?.scrollIntoView({ behavior: 'smooth' }); }}
						class="block text-center text-xs py-3 px-4 bg-[--color-accent] text-white rounded-lg hover:bg-[--color-accent-hover] transition-colors"
					>
						get started →
					</a>
					<p class="text-[10px] text-[--color-text-muted] text-center mt-3">Cancel anytime · powered by Lemon Squeezy</p>
				</div>
			</div>

			<!-- Unlimited -->
			<div class="border border-[--color-border] rounded-xl p-7 flex flex-col">
				<div class="mb-7">
					<p class="text-xs text-[--color-text-muted] uppercase tracking-widest mb-2">Unlimited</p>
					<p class="text-4xl font-semibold text-[--color-text] mb-1">$100</p>
					<p class="text-xs text-[--color-text-muted]">per month · billed via Lemon Squeezy</p>
				</div>

				<div class="space-y-5 flex-1">
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Infrastructure</p>
						<ul class="space-y-1.5 text-xs">
							<li class="text-[--color-text]">Dedicated Rubix node</li>
							<li class="text-[--color-text]">Own IPFS peer ID</li>
							<li class="text-[--color-text]">Own P2P identity on the network</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">API Access</p>
						<ul class="space-y-1.5 text-xs">
							<li class="text-[--color-text]">No request limit</li>
							<li class="text-[--color-text-dim]">Multiple API keys</li>
							<li class="text-[--color-text-dim]">Never rate-limited by quota</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Webhooks</p>
						<ul class="space-y-1.5 text-xs">
							<li class="text-[--color-text]">Unlimited subscriptions</li>
							<li class="text-[--color-text-dim]">All 4 event types</li>
							<li class="text-[--color-text-dim]">HMAC-signed payloads</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Smart Contracts</p>
						<ul class="space-y-1.5 text-xs">
							<li class="text-[--color-text]">Unlimited hosted contracts</li>
							<li class="text-[--color-text-dim]">Full execution log</li>
						</ul>
					</div>
					<div>
						<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest mb-2">Signing</p>
						<ul class="space-y-1.5 text-xs text-[--color-text-dim]">
							<li>Non-custodial · your key always</li>
						</ul>
					</div>
				</div>

				<div class="mt-7">
					<a
						href="#hero"
						onclick={(e) => { e.preventDefault(); document.getElementById('hero')?.scrollIntoView({ behavior: 'smooth' }); }}
						class="block text-center text-xs py-3 px-4 border border-[--color-border] text-[--color-text-dim] rounded-lg hover:border-[--color-accent] hover:text-[--color-accent] transition-colors"
					>
						get started →
					</a>
					<p class="text-[10px] text-[--color-text-muted] text-center mt-3">Cancel anytime · powered by Lemon Squeezy</p>
				</div>
			</div>
		</div>
	</section>

	<!-- ── CTA strip ─────────────────────────────────────────────────────── -->
	<section class="py-20 text-center border-t border-[--color-border]">
		<h2 class="text-2xl font-semibold text-[--color-text] mb-3">
			Start building on Rubix in 30 seconds.
		</h2>
		<p class="text-xs text-[--color-text-muted] mb-8">
			No credit card. No email. Just Telegram.
		</p>
		<a
			href="#hero"
			onclick={(e) => { e.preventDefault(); document.getElementById('hero')?.scrollIntoView({ behavior: 'smooth' }); }}
			class="inline-block text-sm py-3 px-8 bg-[--color-accent] text-white rounded-lg hover:bg-[--color-accent-hover] transition-colors"
		>
			Login with Telegram
		</a>
	</section>

	<!-- ── Footer ─────────────────────────────────────────────────────────── -->
	<footer class="border-t border-[--color-border] py-8 px-6">
		<div class="max-w-5xl mx-auto flex items-center justify-between text-xs text-[--color-text-muted]">
			<span>node3.cloud</span>
			<div class="flex gap-6">
				<a href="/docs" class="hover:text-[--color-text] transition-colors">docs</a>
				<a href="https://rubixchain.github.io/learn/" target="_blank" rel="noopener noreferrer" class="hover:text-[--color-text] transition-colors">Rubix</a>
			</div>
		</div>
	</footer>

</div>
