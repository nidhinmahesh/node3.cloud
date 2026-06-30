<script lang="ts">
	import { onMount } from 'svelte';
	import { auth } from '$lib/auth.svelte';
	import { api, type BillingInfo, type Usage } from '$lib/api';

	let billing = $state<BillingInfo | null>(null);
	let usage = $state<Usage | null>(null);
	let loading = $state(true);
	let error = $state('');
	let checkingOut = $state(false);
	let cancelling = $state(false);

	onMount(async () => {
		try {
			[billing, usage] = await Promise.all([api.billing.get(), api.usage.get()]);
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	});

	async function handleUpgrade() {
		checkingOut = true;
		try {
			const res = await api.billing.checkout();
			window.location.href = res.url;
		} catch (e: any) {
			error = e.message;
			checkingOut = false;
		}
	}

	async function handleCancel() {
		if (!confirm("Cancel your Pro subscription? You'll be downgraded at the end of the billing period.")) return;
		cancelling = true;
		try {
			await api.billing.cancel();
			billing = await api.billing.get();
		} catch (e: any) {
			error = e.message;
		} finally {
			cancelling = false;
		}
	}

	function fmtDate(s: string | null) {
		if (!s) return '—';
		return new Date(s).toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' });
	}

	function usagePct(u: Usage) {
		if (!u.limit) return 0;
		return Math.min(100, Math.round((u.request_count / u.limit) * 100));
	}
</script>

<div class="p-8 max-w-2xl">
	<h1 class="text-sm font-medium text-[--color-text] mb-8">billing</h1>

	{#if error}
		<p class="text-xs text-[--color-red] mb-6">{error}</p>
	{/if}

	{#if loading}
		<p class="text-xs text-[--color-text-muted]">loading…</p>
	{:else}
		<!-- Current plan -->
		<div class="border border-[--color-border] rounded-lg p-5 mb-6">
			<p class="text-[10px] text-[--color-text-muted] mb-3 uppercase tracking-widest">Current Plan</p>
			<div class="flex items-center justify-between">
				<div>
					<p class="text-lg font-semibold text-[--color-text] capitalize">
						{billing?.tier ?? auth.user?.tier ?? 'free'}
					</p>
					{#if billing?.tier === 'paid'}
						<p class="text-xs text-[--color-text-muted] mt-1">
							{billing.cancel_at
								? `Cancels ${fmtDate(billing.cancel_at)}`
								: `Next billing ${fmtDate(billing.next_billing_date)}`}
						</p>
					{:else}
						<p class="text-xs text-[--color-text-muted] mt-1">
							Shared node · 10,000 requests/month · 3 webhooks · 1 contract
						</p>
					{/if}
				</div>
				{#if billing?.tier === 'paid'}
					<span class="text-xs px-2 py-1 border border-[--color-accent] text-[--color-accent] rounded">
						Pro
					</span>
				{/if}
			</div>
		</div>

		<!-- Usage summary -->
		{#if usage}
			<div class="border border-[--color-border] rounded-lg p-5 mb-6">
				<p class="text-[10px] text-[--color-text-muted] mb-3 uppercase tracking-widest">This Month</p>
				<div class="flex items-baseline gap-2 mb-3">
					<span class="text-xl font-semibold text-[--color-text]">
						{usage.request_count.toLocaleString()}
					</span>
					<span class="text-xs text-[--color-text-muted]">
						/ {usage.limit.toLocaleString()} requests
					</span>
				</div>
				<div class="h-1.5 bg-[--color-bg-surface] rounded-full overflow-hidden mb-2">
					<div
						class="h-full rounded-full transition-all
							{usagePct(usage) >= 90 ? 'bg-[--color-red]' : usagePct(usage) >= 70 ? 'bg-[--color-yellow]' : 'bg-[--color-accent]'}"
						style="width: {usagePct(usage)}%"
					></div>
				</div>
				<p class="text-[10px] text-[--color-text-muted]">
					Resets {fmtDate(usage.reset_at)}
				</p>
			</div>
		{/if}

		<!-- Upgrade / manage — use auth.user.tier as fallback when billing API failed -->
		{#if (billing?.tier ?? auth.user?.tier ?? 'free') === 'free'}
			<div class="border border-[--color-accent] rounded-lg p-5">
				<p class="text-[10px] text-[--color-accent] mb-3 uppercase tracking-widest">Upgrade to Pro</p>
				<ul class="space-y-2 text-xs text-[--color-text-dim] mb-5">
					<li>● Dedicated node — your own IPFS peer ID and P2P identity</li>
					<li>● Higher request limits</li>
					<li>● Unlimited webhook subscriptions</li>
					<li>● Multiple hosted smart contracts</li>
				</ul>
				<button
					onclick={handleUpgrade}
					disabled={checkingOut}
					class="w-full text-xs py-2.5 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors disabled:opacity-50"
				>
					{checkingOut ? 'redirecting to checkout…' : 'upgrade to Pro →'}
				</button>
				<p class="text-[10px] text-[--color-text-muted] text-center mt-3">
					Powered by Lemon Squeezy · cancel anytime
				</p>
			</div>
		{:else if (billing?.tier ?? auth.user?.tier) === 'paid'}
			<div class="border border-[--color-border] rounded-lg p-5">
				<p class="text-[10px] text-[--color-text-muted] mb-4 uppercase tracking-widest">Manage Subscription</p>
				{#if billing?.cancel_at}
					<p class="text-xs text-[--color-yellow] mb-4">
						Your subscription is scheduled to cancel on {fmtDate(billing.cancel_at)}.
						You'll retain Pro access until then.
					</p>
				{:else}
					<button
						onclick={handleCancel}
						disabled={cancelling}
						class="text-xs text-[--color-text-muted] hover:text-[--color-red] transition-colors"
					>
						{cancelling ? 'cancelling…' : 'cancel subscription'}
					</button>
				{/if}
			</div>
		{/if}
	{/if}
</div>
