<script lang="ts">
	import { onMount } from 'svelte';
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

		// Telegram Login Widget (callback mode)
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

		const script = document.createElement('script');
		script.src = 'https://telegram.org/js/telegram-widget.js?22';
		script.setAttribute('data-telegram-login', botUsername);
		script.setAttribute('data-size', 'large');
		script.setAttribute('data-onauth', 'onTelegramAuth(user)');
		script.setAttribute('data-request-access', 'write');
		script.async = true;
		telegramContainer?.appendChild(script);
	});
</script>

<div class="min-h-screen bg-[--color-bg] text-[--color-text]">

	<!-- ── Hero ─────────────────────────────────────────────────────────── -->
	<section id="hero" class="min-h-screen flex flex-col items-center justify-center px-6 text-center">
		<p class="text-xs text-[--color-text-muted] mb-6 tracking-widest uppercase">Rubix Network</p>

		<h1 class="text-4xl md:text-6xl font-semibold text-[--color-text] leading-tight mb-4 max-w-3xl">
			Build on Rubix.<br />Without the infrastructure.
		</h1>

		<p class="text-base text-[--color-text-dim] mb-10 max-w-xl leading-relaxed">
			A managed node platform with webhooks, non-custodial signing,
			and hosted smart contracts. Free to start.
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

			<a
				href="#features"
				class="text-xs text-[--color-text-muted] hover:text-[--color-text] transition-colors mt-2"
			>
				learn more ↓
			</a>
		</div>
	</section>

	<!-- ── Problem statement ─────────────────────────────────────────────── -->
	<section id="features" class="max-w-2xl mx-auto px-6 py-20 text-center">
		<p class="text-base text-[--color-text-dim] leading-relaxed">
			Running a Rubix node means managing IPFS, P2P networking, port forwarding,
			and a database — before writing a single line of your app.
		</p>
		<p class="text-base text-[--color-text-dim] leading-relaxed mt-4">
			node3.cloud does all of that so you don't have to.
		</p>
	</section>

	<!-- ── Feature 1 — Webhooks ─────────────────────────────────────────── -->
	<section class="max-w-3xl mx-auto px-6 py-16">
		<div class="grid md:grid-cols-2 gap-12 items-center">
			<div>
				<p class="text-xs text-[--color-accent] mb-3 tracking-widest uppercase">Webhooks</p>
				<h2 class="text-2xl font-semibold text-[--color-text] mb-4 leading-snug">
					Know when it happens.<br />Not when you check.
				</h2>
				<p class="text-sm text-[--color-text-dim] leading-relaxed">
					Subscribe to any event on the Rubix network — a transfer to your DID,
					a smart contract execution, a new token mint — and get an HTTP POST
					to your server the moment it settles.
					No polling. No missed events.
				</p>
			</div>
			<div class="bg-[--color-bg-surface] border border-[--color-border] rounded-lg p-4 text-xs font-mono leading-relaxed">
				<p class="text-[--color-text-muted] mb-2">POST https://your-app.com/hooks/rubix</p>
				<pre class="text-[--color-text-dim] whitespace-pre-wrap">{`{
  "event": "token.received",
  "data": {
    "to_did": "bafybmi...",
    "amount": 1.5,
    "transaction_id": "..."
  }
}`}</pre>
			</div>
		</div>
	</section>

	<!-- ── Feature 2 — Non-custodial ────────────────────────────────────── -->
	<section class="max-w-3xl mx-auto px-6 py-16">
		<div class="grid md:grid-cols-2 gap-12 items-center">
			<div class="order-2 md:order-1">
				<div class="flex gap-4 items-center justify-center text-xs text-[--color-text-dim]">
					<div class="text-center">
						<div class="w-24 h-16 border border-[--color-border] rounded flex items-center justify-center mb-2 text-[--color-accent]">
							your device
						</div>
						<p>key lives here</p>
					</div>
					<div class="text-[--color-text-muted]">→</div>
					<div class="text-center">
						<div class="w-24 h-16 border border-[--color-border] rounded flex items-center justify-center mb-2 text-[--color-text-muted]">
							node3.cloud
						</div>
						<p>never signs</p>
					</div>
				</div>
			</div>
			<div class="order-1 md:order-2">
				<p class="text-xs text-[--color-accent] mb-3 tracking-widest uppercase">Non-Custodial</p>
				<h2 class="text-2xl font-semibold text-[--color-text] mb-4 leading-snug">
					Your keys.<br />Always.
				</h2>
				<p class="text-sm text-[--color-text-dim] leading-relaxed">
					Your private key is derived from your secret phrase in your browser
					and never leaves your device. node3.cloud manages your node on the
					network but cannot sign a single transaction without you.
					Most hosted node services hold your keys. We don't.
				</p>
			</div>
		</div>
	</section>

	<!-- ── Feature 3 — Smart contracts ─────────────────────────────────── -->
	<section class="max-w-3xl mx-auto px-6 py-16">
		<div class="grid md:grid-cols-2 gap-12 items-center">
			<div>
				<p class="text-xs text-[--color-accent] mb-3 tracking-widest uppercase">Smart Contracts</p>
				<h2 class="text-2xl font-semibold text-[--color-text] mb-4 leading-snug">
					Deploy a contract.<br />Not a server.
				</h2>
				<p class="text-sm text-[--color-text-dim] leading-relaxed">
					Rubix smart contracts need a running callback server to handle executions.
					node3.cloud hosts it for you — upload your WASM,
					and every execution runs on our infrastructure with full logs,
					state history, and webhook events on each run.
				</p>
			</div>
			<div class="bg-[--color-bg-surface] border border-[--color-border] rounded-lg p-4 text-xs font-mono leading-relaxed">
				<p class="text-[--color-text-muted] mb-2">POST /api/contracts/deploy</p>
				<pre class="text-[--color-text-dim] whitespace-pre-wrap">{`{
  "wasm": "<base64>",
  "initial_state": { "count": 0 }
}`}</pre>
				<p class="text-[--color-green] mt-3">→ Contract live. Executions logged.</p>
			</div>
		</div>
	</section>

	<!-- ── Pricing ───────────────────────────────────────────────────────── -->
	<section class="max-w-2xl mx-auto px-6 py-20 text-center">
		<h2 class="text-xl font-semibold text-[--color-text] mb-10">Simple pricing.</h2>
		<div class="grid md:grid-cols-2 gap-6">
			<!-- Free -->
			<div class="border border-[--color-border] rounded-lg p-6 text-left">
				<p class="text-xs text-[--color-text-muted] mb-1">Free</p>
				<p class="text-2xl font-semibold text-[--color-text] mb-6">$0 <span class="text-sm font-normal text-[--color-text-muted]">/ month</span></p>
				<ul class="space-y-2 text-xs text-[--color-text-dim] mb-8">
					<li>1 DID on shared node</li>
					<li>10,000 requests / month</li>
					<li>3 webhook subscriptions</li>
					<li>1 hosted contract</li>
				</ul>
				<a
					href="#hero"
					onclick={(e) => { e.preventDefault(); document.getElementById('hero')?.scrollIntoView({ behavior: 'smooth' }); }}
					class="block text-center text-xs py-2 px-4 border border-[--color-accent] text-[--color-accent] rounded hover:bg-[--color-accent-dim] transition-colors"
				>
					get started →
				</a>
			</div>
			<!-- Pro -->
			<div class="border border-[--color-accent] rounded-lg p-6 text-left bg-[--color-bg-surface]">
				<p class="text-xs text-[--color-accent] mb-1">Pro</p>
				<p class="text-2xl font-semibold text-[--color-text] mb-6">
					<span class="text-sm font-normal text-[--color-text-muted]">coming soon</span>
				</p>
				<ul class="space-y-2 text-xs text-[--color-text-dim] mb-8">
					<li>Dedicated node (own P2P identity)</li>
					<li>Higher request limits</li>
					<li>Unlimited webhooks</li>
					<li>Multiple hosted contracts</li>
				</ul>
				<button
					disabled
					class="w-full text-xs py-2 px-4 border border-[--color-border] text-[--color-text-muted] rounded cursor-not-allowed"
				>
					upgrade →
				</button>
			</div>
		</div>
	</section>

	<!-- ── Bottom CTA ────────────────────────────────────────────────────── -->
	<section class="py-20 text-center border-t border-[--color-border]">
		<h2 class="text-xl font-semibold text-[--color-text] mb-3">
			Start building on Rubix in 30 seconds.
		</h2>
		<p class="text-xs text-[--color-text-muted] mb-8">
			No credit card required. No email. Just Telegram.
		</p>
		<a
			href="#hero"
			onclick={(e) => { e.preventDefault(); document.getElementById('hero')?.scrollIntoView({ behavior: 'smooth' }); }}
			class="inline-block text-sm py-2.5 px-6 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors"
		>
			Login with Telegram
		</a>
	</section>

	<!-- ── Footer ────────────────────────────────────────────────────────── -->
	<footer class="border-t border-[--color-border] py-8 px-6">
		<div class="max-w-3xl mx-auto text-xs text-[--color-text-muted]">
			<span>node3.cloud</span>
		</div>
	</footer>

</div>
