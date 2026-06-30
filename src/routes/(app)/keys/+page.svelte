<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Key } from '$lib/api';
	import Modal from '$lib/components/Modal.svelte';

	let keys = $state<Key[]>([]);
	let loading = $state(true);
	let error = $state('');

	// create modal
	let showCreate = $state(false);
	let newLabel = $state('');
	let creating = $state(false);
	let createError = $state('');
	let createdKey = $state('');

	// revoke
	let revoking = $state<string | null>(null);

	// copy feedback
	let copied = $state('');

	onMount(async () => {
		await load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			keys = await api.keys.list();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function handleCreate() {
		if (!newLabel.trim()) return;
		creating = true;
		createError = '';
		try {
			const res = await api.keys.create(newLabel.trim());
			createdKey = res.key;
			newLabel = '';
			await load();
		} catch (e: any) {
			createError = e.message;
		} finally {
			creating = false;
		}
	}

	async function handleRevoke(id: string) {
		if (!confirm('Revoke this key? Apps using it will stop working immediately.')) return;
		revoking = id;
		try {
			await api.keys.revoke(id);
			await load();
		} catch (e: any) {
			error = e.message;
		} finally {
			revoking = null;
		}
	}

	function copyKey(val: string, id: string) {
		navigator.clipboard.writeText(val);
		copied = id;
		setTimeout(() => { copied = ''; }, 1500);
	}

	function fmtDate(s: string) {
		return new Date(s).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
	}

	const activeKeys = $derived(keys.filter(k => !k.revoked_at));
	const revokedKeys = $derived(keys.filter(k => k.revoked_at));
</script>

<div class="p-8 max-w-3xl">
	<div class="flex items-center justify-between mb-8">
		<h1 class="text-sm font-medium text-[--color-text]">api keys</h1>
		<button
			onclick={() => { showCreate = true; createdKey = ''; createError = ''; }}
			class="text-xs px-3 py-1.5 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors"
		>
			+ new key
		</button>
	</div>

	{#if error}
		<p class="text-xs text-[--color-red] mb-4">{error}</p>
	{/if}

	<!-- Active keys -->
	{#if loading}
		<p class="text-xs text-[--color-text-muted]">loading…</p>
	{:else if activeKeys.length === 0}
		<div class="border border-[--color-border] rounded-lg p-8 text-center">
			<p class="text-xs text-[--color-text-muted]">No active API keys.</p>
			<button
				onclick={() => { showCreate = true; }}
				class="mt-4 text-xs text-[--color-accent] hover:underline"
			>
				Create your first key →
			</button>
		</div>
	{:else}
		<div class="space-y-3 mb-8">
			{#each activeKeys as key}
				<div class="border border-[--color-border] rounded-lg p-4">
					<div class="flex items-center justify-between mb-2">
						<p class="text-xs font-medium text-[--color-text]">{key.label || '(unlabelled)'}</p>
						<div class="flex items-center gap-3">
							<span class="text-[10px] text-[--color-text-muted]">
								created {fmtDate(key.created_at)}
							</span>
							<button
								onclick={() => handleRevoke(key.id)}
								disabled={revoking === key.id}
								class="text-[10px] text-[--color-text-muted] hover:text-[--color-red] transition-colors"
							>
								{revoking === key.id ? 'revoking…' : 'revoke'}
							</button>
						</div>
					</div>
					<p class="text-[10px] text-[--color-text-muted]">
						{key.last_used_at ? `last used ${fmtDate(key.last_used_at)}` : 'never used'}
					</p>
				</div>
			{/each}
		</div>
	{/if}

	<!-- Revoked keys (collapsed) -->
	{#if revokedKeys.length > 0}
		<details class="text-xs text-[--color-text-muted]">
			<summary class="cursor-pointer hover:text-[--color-text] mb-3 transition-colors">
				{revokedKeys.length} revoked {revokedKeys.length === 1 ? 'key' : 'keys'}
			</summary>
			<div class="space-y-2">
				{#each revokedKeys as key}
					<div class="border border-[--color-border] rounded p-3 opacity-50">
						<p>{key.label || '(unlabelled)'} — revoked {fmtDate(key.revoked_at!)}</p>
					</div>
				{/each}
			</div>
		</details>
	{/if}
</div>

<!-- Create modal -->
<Modal bind:open={showCreate} title="create api key">
	{#if createdKey}
		<p class="text-xs text-[--color-text-dim] mb-4 leading-relaxed">
			Copy your key now. It won't be shown again.
		</p>
		<div class="flex items-center gap-2 bg-[--color-bg] border border-[--color-border] rounded p-3 mb-4">
			<code class="text-xs text-[--color-accent] flex-1 break-all">{createdKey}</code>
			<button
				onclick={() => copyKey(createdKey, 'new')}
				class="text-[10px] text-[--color-text-muted] hover:text-[--color-text] border border-[--color-border] px-2 py-1 rounded shrink-0 transition-colors"
			>
				{copied === 'new' ? 'copied!' : 'copy'}
			</button>
		</div>
		<button
			onclick={() => { showCreate = false; createdKey = ''; }}
			class="w-full text-xs py-2 border border-[--color-border] rounded text-[--color-text-dim] hover:text-[--color-text] transition-colors"
		>
			done
		</button>
	{:else}
		<div class="space-y-4">
			<div>
				<label class="block text-[10px] text-[--color-text-muted] mb-1.5 uppercase tracking-widest">
					label
				</label>
				<input
					type="text"
					bind:value={newLabel}
					placeholder="e.g. my dapp prod"
					class="w-full"
					onkeydown={(e) => e.key === 'Enter' && handleCreate()}
				/>
			</div>
			{#if createError}
				<p class="text-xs text-[--color-red]">{createError}</p>
			{/if}
			<button
				onclick={handleCreate}
				disabled={creating || !newLabel.trim()}
				class="w-full text-xs py-2 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
			>
				{creating ? 'creating…' : 'create key'}
			</button>
		</div>
	{/if}
</Modal>
