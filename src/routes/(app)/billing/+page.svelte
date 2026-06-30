<script lang="ts">
	import { onMount } from 'svelte';
	import { auth } from '$lib/auth.svelte';
	import { api, type BillingInfo, type Usage } from '$lib/api';

	let billing = $state<BillingInfo | null>(null);
	let usage = $state<Usage | null>(null);
	let loading = $state(true);
	let error = $state('');
	let checkingOut = $state<'pro' | 'unlimited' | null>(null);
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

	async function handleUpgrade(plan: 'pro' | 'unlimited') {
		checkingOut = plan;
		try {
			const res = await api.billing.checkout(plan);
			window.location.href = res.url;
		} catch (e: any) {
			error = e.message;
			checkingOut = null;
		}
	}

	async function handleCancel() {
		if (!confirm("Cancel your subscription? You'll be downgraded at the end of the billing period.")) return;
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

	const tier = $derived(billing?.tier ?? auth.user?.tier ?? 'free');
	const isFree      = $derived(tier === 'free');
	const isPro       = $derived(tier === 'pro');
	const isUnlimited = $derived(tier === 'unlimited');
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
					<p class="text-lg font-semibold text-[--color-text] capitalize">{tier}</p>
					{#if isFree}
						<p class="text-xs text-[--color-text-muted] mt-1">
							Shared node · 10,000 requests/month · 3 webhooks · 1 contract
						</p>
					{:else if isPro}
						<p class="text-xs text-[--color-text-muted] mt-1">
							{billing?.cancel_at
								? `Cancels ${fmtDate(billing.cancel_at)}`
								: `Next billing ${fmtDate(billing?.next_billing_date ?? null)}`}
						</p>
						<p class="text-xs text-[--color-text-muted] mt-0.5">
							Dedicated node · 500,000 requests/month · unlimited webhooks & contracts
						</p>
					{:else if isUnlimited}
						<p class="text-xs text-[--color-text-muted] mt-1">
							{billing?.cancel_at
								? `Cancels ${fmtDate(billing.cancel_at)}`
								: `Next billing ${fmtDate(billing?.next_billing_date ?? null)}`}
						</p>
						<p class="text-xs text-[--color-text-muted] mt-0.5">
							Dedicated node · unlimited requests · unlimited webhooks & contracts
						</p>
					{/if}
				</div>
				{#if isPro}
					<span class="text-xs px-2 py-1 border border-[--color-accent] text-[--color-accent] rounded">Pro</span>
				{:else if isUnlimited}
					<span class="text-xs px-2 py-1 border border-[--color-accent] text-[--color-accent] rounded">Unlimited</span>
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
						/ {isUnlimited ? 'unlimited' : usage.limit.toLocaleString()} requests
					</span>
				</div>
				{#if !isUnlimited}
					<div class="h-1.5 bg-[--color-bg-surface] rounded-full overflow-hidden mb-2">
						<div
							class="h-full rounded-full transition-all
								{usagePct(usage) >= 90 ? 'bg-[--color-red]' : usagePct(usage) >= 70 ? 'bg-[--color-yellow]' : 'bg-[--color-accent]'}"
							style="width: {usagePct(usage)}%"
						></div>
					</div>
				{/if}
				<p class="text-[10px] text-[--color-text-muted]">Resets {fmtDate(usage.reset_at)}</p>
			</div>
		{/if}

		<!-- Upgrade / manage -->
		{#if isFree}
			<div class="grid md:grid-cols-2 gap-4">
				<!-- Upgrade to Pro -->
				<div class="border border-[--color-accent] rounded-lg p-5">
					<p class="text-[10px] text-[--color-accent] mb-1 uppercase tracking-widest">Pro</p>
					<p class="text-2xl font-semibold text-[--color-text] mb-3">$30 <span class="text-xs font-normal text-[--color-text-muted]">/ month</span></p>
					<ul class="space-y-1.5 text-xs text-[--color-text-dim] mb-5">
						<li>● Dedicated Rubix node</li>
						<li>● 500,000 requests / month</li>
						<li>● Unlimited webhooks</li>
						<li>● Unlimited contracts</li>
					</ul>
					<button
						onclick={() => handleUpgrade('pro')}
						disabled={checkingOut !== null}
						class="w-full text-xs py-2.5 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors disabled:opacity-50"
					>
						{checkingOut === 'pro' ? 'redirecting…' : 'upgrade to Pro →'}
					</button>
				</div>

				<!-- Upgrade to Unlimited -->
				<div class="border border-[--color-border] rounded-lg p-5">
					<p class="text-[10px] text-[--color-text-muted] mb-1 uppercase tracking-widest">Unlimited</p>
					<p class="text-2xl font-semibold text-[--color-text] mb-3">$100 <span class="text-xs font-normal text-[--color-text-muted]">/ month</span></p>
					<ul class="space-y-1.5 text-xs text-[--color-text-dim] mb-5">
						<li>● Dedicated Rubix node</li>
						<li>● No request limit</li>
						<li>● Unlimited webhooks</li>
						<li>● Unlimited contracts</li>
					</ul>
					<button
						onclick={() => handleUpgrade('unlimited')}
						disabled={checkingOut !== null}
						class="w-full text-xs py-2.5 border border-[--color-border] text-[--color-text-dim] rounded hover:border-[--color-accent] hover:text-[--color-accent] transition-colors disabled:opacity-50"
					>
						{checkingOut === 'unlimited' ? 'redirecting…' : 'upgrade to Unlimited →'}
					</button>
				</div>
			</div>
			<p class="text-[10px] text-[--color-text-muted] text-center mt-3">Powered by Lemon Squeezy · cancel anytime</p>

		{:else if isPro}
			<div class="border border-[--color-border] rounded-lg p-5">
				<p class="text-[10px] text-[--color-text-muted] mb-4 uppercase tracking-widest">Manage Subscription</p>
				{#if billing?.cancel_at}
					<p class="text-xs text-[--color-yellow] mb-4">
						Scheduled to cancel on {fmtDate(billing.cancel_at)}. Pro access continues until then.
					</p>
				{:else}
					<div class="flex items-center justify-between">
						<button
							onclick={() => handleUpgrade('unlimited')}
							disabled={checkingOut !== null}
							class="text-xs px-3 py-1.5 border border-[--color-accent] text-[--color-accent] rounded hover:bg-[--color-accent] hover:text-white transition-colors disabled:opacity-50"
						>
							{checkingOut === 'unlimited' ? 'redirecting…' : 'upgrade to Unlimited · $100/mo →'}
						</button>
						<button
							onclick={handleCancel}
							disabled={cancelling}
							class="text-xs text-[--color-text-muted] hover:text-[--color-red] transition-colors"
						>
							{cancelling ? 'cancelling…' : 'cancel subscription'}
						</button>
					</div>
				{/if}
			</div>

		{:else if isUnlimited}
			<div class="border border-[--color-border] rounded-lg p-5">
				<p class="text-[10px] text-[--color-text-muted] mb-4 uppercase tracking-widest">Manage Subscription</p>
				{#if billing?.cancel_at}
					<p class="text-xs text-[--color-yellow] mb-4">
						Scheduled to cancel on {fmtDate(billing.cancel_at)}. Unlimited access continues until then.
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
