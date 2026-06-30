<script lang="ts">
	import { onMount } from 'svelte';
	import { auth } from '$lib/auth.svelte';
	import { api, type Webhook, type Delivery } from '$lib/api';
	import Modal from '$lib/components/Modal.svelte';

	const EVENT_TYPES = [
		{ value: 'token.received', label: 'token.received — token lands in a watched DID' },
		{ value: 'token.sent', label: 'token.sent — token leaves a watched DID' },
		{ value: 'contract.deployed', label: 'contract.deployed — contract genesis block' },
		{ value: 'contract.executed', label: 'contract.executed — contract execution' }
	];

	let webhooks = $state<Webhook[]>([]);
	let loading = $state(true);
	let error = $state('');

	// create modal
	let showCreate = $state(false);
	let form = $state({ event_type: 'token.received', filter_value: '', callback_url: '' });
	let creating = $state(false);

	// deliveries modal
	let showDeliveries = $state(false);
	let selectedWebhook = $state<Webhook | null>(null);
	let deliveries = $state<Delivery[]>([]);
	let loadingDeliveries = $state(false);
	let deliveriesError = $state('');

	let removing = $state<string | null>(null);

	onMount(load);

	async function load() {
		loading = true;
		error = '';
		try {
			webhooks = await api.webhooks.list();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function handleCreate() {
		if (!form.filter_value.trim() || !form.callback_url.trim()) return;
		creating = true;
		try {
			await api.webhooks.create({
				event_type: form.event_type,
				filter_value: form.filter_value.trim(),
				callback_url: form.callback_url.trim()
			});
			showCreate = false;
			form = { event_type: 'token.received', filter_value: '', callback_url: '' };
			await load();
		} catch (e: any) {
			error = e.message;
		} finally {
			creating = false;
		}
	}

	async function handleRemove(id: string) {
		if (!confirm('Delete this webhook subscription?')) return;
		removing = id;
		try {
			await api.webhooks.remove(id);
			await load();
		} catch (e: any) {
			error = e.message;
		} finally {
			removing = null;
		}
	}

	async function openDeliveries(hook: Webhook) {
		selectedWebhook = hook;
		deliveriesError = '';
		deliveries = [];
		showDeliveries = true;
		loadingDeliveries = true;
		try {
			deliveries = await api.webhooks.deliveries(hook.id);
		} catch (e: any) {
			deliveriesError = e.message || 'Failed to load delivery history.';
		} finally {
			loadingDeliveries = false;
		}
	}

	function fmtDate(s: string) {
		return new Date(s).toLocaleString('en-US', {
			month: 'short', day: 'numeric',
			hour: '2-digit', minute: '2-digit'
		});
	}

	const FREE_LIMIT = 3;
	const isPaid = $derived(auth.user?.tier === 'pro' || auth.user?.tier === 'unlimited');
	const atLimit = $derived(!isPaid && webhooks.filter(w => w.active).length >= FREE_LIMIT);
</script>

<div class="p-8 max-w-3xl">
	<div class="flex items-center justify-between mb-8">
		<h1 class="text-sm font-medium text-[--color-text]">webhooks</h1>
		<button
			onclick={() => { showCreate = true; }}
			disabled={atLimit}
			class="text-xs px-3 py-1.5 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
			title={atLimit ? 'Free tier limit: 3 active webhooks' : ''}
		>
			+ subscribe
		</button>
	</div>

	{#if atLimit}
		<p class="text-xs text-[--color-yellow] mb-4">
			Free tier: 3 webhook limit reached. Upgrade to Pro for unlimited subscriptions.
		</p>
	{/if}

	{#if error}
		<p class="text-xs text-[--color-red] mb-4">{error}</p>
	{/if}

	<!-- Webhook list -->
	{#if loading}
		<p class="text-xs text-[--color-text-muted]">loading…</p>
	{:else if webhooks.length === 0}
		<div class="border border-[--color-border] rounded-lg p-8 text-center">
			<p class="text-xs text-[--color-text-muted] mb-2">No webhook subscriptions.</p>
			<p class="text-xs text-[--color-text-muted]">
				Subscribe to network events and receive HTTP callbacks when they happen.
			</p>
			<button
				onclick={() => { showCreate = true; }}
				class="mt-4 text-xs text-[--color-accent] hover:underline"
			>
				Add your first subscription →
			</button>
		</div>
	{:else}
		<div class="space-y-3">
			{#each webhooks as hook}
				<div class="border border-[--color-border] rounded-lg p-4">
					<div class="flex items-start justify-between gap-4">
						<div class="flex-1 min-w-0">
							<div class="flex items-center gap-2 mb-2">
								<span class="text-[10px] px-1.5 py-0.5 rounded bg-[--color-bg-surface] border border-[--color-border] text-[--color-accent]">
									{hook.event_type}
								</span>
								<span class="text-[10px] {hook.active ? 'text-[--color-green]' : 'text-[--color-text-muted]'}">
									{hook.active ? '● active' : '○ paused'}
								</span>
							</div>
							<p class="text-xs text-[--color-text-dim] truncate mb-1" title={hook.filter_value}>
								filter: <span class="font-mono">{hook.filter_value}</span>
							</p>
							<p class="text-xs text-[--color-text-muted] truncate" title={hook.callback_url}>
								→ {hook.callback_url}
							</p>
						</div>
						<div class="flex gap-3 shrink-0">
							<button
								onclick={() => openDeliveries(hook)}
								class="text-[10px] text-[--color-text-muted] hover:text-[--color-text] transition-colors"
							>
								history
							</button>
							<button
								onclick={() => handleRemove(hook.id)}
								disabled={removing === hook.id}
								class="text-[10px] text-[--color-text-muted] hover:text-[--color-red] transition-colors"
							>
								{removing === hook.id ? 'removing…' : 'remove'}
							</button>
						</div>
					</div>
					<p class="text-[10px] text-[--color-text-muted] mt-2">created {fmtDate(hook.created_at)}</p>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Create modal -->
<Modal bind:open={showCreate} title="add webhook subscription">
	<div class="space-y-4">
		<div>
			<label class="block text-[10px] text-[--color-text-muted] mb-1.5 uppercase tracking-widest">Event</label>
			<select bind:value={form.event_type} class="w-full">
				{#each EVENT_TYPES as et}
					<option value={et.value}>{et.label}</option>
				{/each}
			</select>
		</div>
		<div>
			<label class="block text-[10px] text-[--color-text-muted] mb-1.5 uppercase tracking-widest">
				{form.event_type.startsWith('contract') ? 'Contract ID' : 'DID to watch'}
			</label>
			<input
				type="text"
				bind:value={form.filter_value}
				placeholder={form.event_type.startsWith('contract') ? 'Qm...' : 'bafybmi...'}
				class="w-full"
			/>
		</div>
		<div>
			<label class="block text-[10px] text-[--color-text-muted] mb-1.5 uppercase tracking-widest">Callback URL</label>
			<input
				type="url"
				bind:value={form.callback_url}
				placeholder="https://your-app.com/webhooks/rubix"
				class="w-full"
			/>
		</div>
		<p class="text-[10px] text-[--color-text-muted] leading-relaxed">
			We'll POST to your URL with an <code>X-Rubix-Signature</code> header for verification.
		</p>
		<button
			onclick={handleCreate}
			disabled={creating || !form.filter_value.trim() || !form.callback_url.trim()}
			class="w-full text-xs py-2 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
		>
			{creating ? 'subscribing…' : 'subscribe'}
		</button>
	</div>
</Modal>

<!-- Deliveries modal -->
<Modal bind:open={showDeliveries} title="delivery history">
	{#if loadingDeliveries}
		<p class="text-xs text-[--color-text-muted]">loading…</p>
	{:else if deliveriesError}
		<p class="text-xs text-[--color-red]">{deliveriesError}</p>
	{:else if deliveries.length === 0}
		<p class="text-xs text-[--color-text-muted]">No deliveries yet.</p>
	{:else}
		<div class="space-y-2 max-h-80 overflow-y-auto">
			{#each deliveries as d}
				<div class="flex items-center justify-between text-xs border-b border-[--color-border] pb-2">
					<div>
						<span class="{d.status === 'success' ? 'text-[--color-green]' : 'text-[--color-red]'}">
							{d.status}
						</span>
						{#if d.response_code}
							<span class="text-[--color-text-muted] ml-2">HTTP {d.response_code}</span>
						{/if}
					</div>
					<span class="text-[--color-text-muted]">{fmtDate(d.attempted_at)}</span>
				</div>
			{/each}
		</div>
	{/if}
</Modal>
