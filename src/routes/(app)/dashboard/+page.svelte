<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import { api, type Usage, type Key, type Webhook } from '$lib/api';

	let usage = $state<Usage | null>(null);
	let keys = $state<Key[]>([]);
	let webhooks = $state<Webhook[]>([]);
	let error = $state('');
	let didError = $state('');

	onMount(async () => {
		try {
			[usage, keys, webhooks] = await Promise.all([
				api.usage.get(),
				api.keys.list(),
				api.webhooks.list()
			]);
		} catch (e: any) {
			error = e.message;
		}
	});

	function usagePct(u: Usage) {
		if (!u.limit) return 0;
		return Math.min(100, Math.round((u.request_count / u.limit) * 100));
	}

	function fmtDate(s: string) {
		return new Date(s).toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
	}

	function copyDid() {
		if (auth.user?.did) navigator.clipboard.writeText(auth.user.did);
	}

	function handleCreateDID() {
		goto('/setup');
	}

	const activeWebhooks = $derived(webhooks.filter(w => w.active).length);
	const activeKeys = $derived(keys.filter(k => !k.revoked_at).length);
</script>

<div class="p-8 max-w-3xl">
	<h1 class="text-sm font-medium text-[--color-text] mb-8">dashboard</h1>

	{#if error}
		<p class="text-xs text-[--color-red] mb-6">{error}</p>
	{/if}

	<!-- DID card -->
	<div class="border border-[--color-border] rounded-lg p-5 mb-6">
		<p class="text-[10px] text-[--color-text-muted] mb-2 uppercase tracking-widest">Your DID</p>
		{#if auth.user?.did}
			<div class="flex items-center gap-3">
				<code class="text-xs text-[--color-text-dim] break-all flex-1">{auth.user.did}</code>
				<button
					onclick={copyDid}
					class="text-[10px] text-[--color-text-muted] hover:text-[--color-text] border border-[--color-border] px-2 py-1 rounded shrink-0 transition-colors"
				>
					copy
				</button>
			</div>
			<p class="text-[10px] text-[--color-text-muted] mt-2">
				{auth.user.tier === 'paid' ? '● dedicated node' : '● shared node'}
			</p>
		{:else}
			<p class="text-xs text-[--color-text-muted] mb-3">
				You don't have a DID yet. Set up your wallet to create one.
			</p>
			<button
				onclick={handleCreateDID}
				class="text-xs border border-[--color-accent] text-[--color-accent] hover:bg-[--color-accent] hover:text-[--color-bg] px-3 py-1.5 rounded transition-colors"
			>
				set up wallet →
			</button>
		{/if}
	</div>

	<!-- Stats row -->
	<div class="grid grid-cols-3 gap-4 mb-6">
		<div class="border border-[--color-border] rounded-lg p-4">
			<p class="text-[10px] text-[--color-text-muted] mb-1 uppercase tracking-widest">API Keys</p>
			<p class="text-2xl font-semibold text-[--color-text]">{activeKeys}</p>
		</div>
		<div class="border border-[--color-border] rounded-lg p-4">
			<p class="text-[10px] text-[--color-text-muted] mb-1 uppercase tracking-widest">Webhooks</p>
			<p class="text-2xl font-semibold text-[--color-text]">{activeWebhooks}</p>
		</div>
		<div class="border border-[--color-border] rounded-lg p-4">
			<p class="text-[10px] text-[--color-text-muted] mb-1 uppercase tracking-widest">Plan</p>
			<p class="text-sm font-semibold text-[--color-text] mt-1 capitalize">{auth.user?.tier ?? '—'}</p>
		</div>
	</div>

	<!-- Usage bar -->
	{#if usage}
		<div class="border border-[--color-border] rounded-lg p-5">
			<div class="flex items-center justify-between mb-3">
				<p class="text-[10px] text-[--color-text-muted] uppercase tracking-widest">Monthly Usage</p>
				<p class="text-[10px] text-[--color-text-muted]">
					resets {fmtDate(usage.reset_at)}
				</p>
			</div>
			<div class="flex items-baseline gap-2 mb-3">
				<span class="text-xl font-semibold text-[--color-text]">
					{usage.request_count.toLocaleString()}
				</span>
				<span class="text-xs text-[--color-text-muted]">
					/ {usage.limit.toLocaleString()} requests
				</span>
			</div>
			<div class="h-1.5 bg-[--color-bg-surface] rounded-full overflow-hidden">
				<div
					class="h-full rounded-full transition-all duration-500
						{usagePct(usage) >= 90 ? 'bg-[--color-red]' : usagePct(usage) >= 70 ? 'bg-[--color-yellow]' : 'bg-[--color-accent]'}"
					style="width: {usagePct(usage)}%"
				></div>
			</div>
			<p class="text-[10px] text-[--color-text-muted] mt-2">{usagePct(usage)}% used</p>
		</div>
	{:else if !error}
		<div class="border border-[--color-border] rounded-lg p-5">
			<div class="h-1.5 bg-[--color-bg-surface] rounded-full"></div>
			<p class="text-[10px] text-[--color-text-muted] mt-2">loading usage…</p>
		</div>
	{/if}
</div>
