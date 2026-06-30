<script lang="ts">
	import { onMount } from 'svelte';
	import { auth } from '$lib/auth.svelte';
	import { api, type Contract, type Execution } from '$lib/api';
	import Modal from '$lib/components/Modal.svelte';
	import { decryptMnemonic, loadMnemonic, deriveKey, signHash } from '$lib/wallet';

	let contracts = $state<Contract[]>([]);
	let loading = $state(true);
	let error = $state('');

	// deploy modal
	let showDeploy = $state(false);
	let wasmFile = $state<File | null>(null);
	let rsFile = $state<File | null>(null);
	let initialState = $state('{}');
	let deploying = $state(false);
	let deployError = $state('');
	let pendingContractId = $state<string | null>(null); // polling for deployed_at

	// signing flow (non-custodial DID)
	let showSign = $state(false);
	let pendingSignId = $state('');
	let pendingHash = $state('');
	let walletPin = $state('');
	let signing = $state(false);
	let signError = $state('');

	// executions modal
	let showExecutions = $state(false);
	let selectedContract = $state<Contract | null>(null);
	let executions = $state<Execution[]>([]);
	let loadingExec = $state(false);
	let execError = $state('');

	onMount(load);

	async function load() {
		loading = true;
		error = '';
		try {
			contracts = await api.contracts.list();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function handleDeploy() {
		if (!wasmFile || !rsFile) return;
		let parsed: unknown;
		try {
			parsed = JSON.parse(initialState);
		} catch {
			deployError = 'Initial state must be valid JSON';
			return;
		}
		deploying = true;
		deployError = '';
		try {
			const form = new FormData();
			form.append('wasm', wasmFile);
			form.append('source', rsFile);
			form.append('initial_state', JSON.stringify(parsed));
			const deployed = await api.contracts.deploy(form);
			showDeploy = false;
			wasmFile = null;
			rsFile = null;
			initialState = '{}';

			if (deployed.needs_signature && deployed.sign_id && deployed.hash) {
				// Non-custodial DID: node needs the wallet's signature to complete deploy.
				pendingSignId = deployed.sign_id;
				pendingHash = deployed.hash;
				showSign = true;
				await load();
				return;
			}

			await load();

			// Custodial path: consensus runs synchronously but deployed_at may not be
			// set yet if the node is still writing state. Poll briefly.
			if (!deployed.deployed_at && deployed.id) {
				pendingContractId = deployed.id;
				let attempts = 0;
				const poll = setInterval(async () => {
					attempts++;
					try { contracts = await api.contracts.list(); } catch { /* ignore */ }
					const found = contracts.find(c => c.id === deployed.id);
					if (found?.deployed_at || attempts >= 10) {
						clearInterval(poll);
						pendingContractId = null;
					}
				}, 3000);
			}
		} catch (e: any) {
			deployError = e.message;
		} finally {
			deploying = false;
		}
	}

	async function handleSign() {
		if (!auth.user) return;
		signing = true;
		signError = '';
		try {
			const blob = await loadMnemonic(auth.user.id);
			if (!blob) throw new Error('No wallet found in this browser. Re-run wallet setup.');
			const mnemonic = await decryptMnemonic(blob, walletPin);
			const { privateKey } = deriveKey(mnemonic);
			const signature = signHash(privateKey, pendingHash);
			privateKey.fill(0);
			await api.tx.sign(pendingSignId, signature);
			showSign = false;
			walletPin = '';
			await load();
		} catch (e: unknown) {
			signError = e instanceof Error ? e.message : 'Signing failed.';
		} finally {
			signing = false;
		}
	}

	async function openExecutions(c: Contract) {
		selectedContract = c;
		execError = '';
		executions = [];
		showExecutions = true;
		loadingExec = true;
		try {
			executions = await api.contracts.executions(c.id);
		} catch (e: any) {
			execError = e.message || 'Failed to load executions.';
		} finally {
			loadingExec = false;
		}
	}

	function fmtDate(s: string | null) {
		if (!s) return '—';
		return new Date(s).toLocaleString('en-US', {
			month: 'short', day: 'numeric',
			hour: '2-digit', minute: '2-digit'
		});
	}

	function fmtJson(v: unknown) {
		try { return JSON.stringify(v, null, 2); } catch { return String(v); }
	}

	const FREE_LIMIT = 1;
	const isPaid = $derived(auth.user?.tier === 'paid');
	const atLimit = $derived(!isPaid && contracts.length >= FREE_LIMIT);
	const execModalTitle = $derived(
		selectedContract ? `executions · ${selectedContract.contract_id.slice(0, 12)}…` : 'execution log'
	);
</script>

<div class="p-8 max-w-3xl">
	<div class="flex items-center justify-between mb-8">
		<h1 class="text-sm font-medium text-[--color-text]">contracts</h1>
		<button
			onclick={() => { showDeploy = true; deployError = ''; }}
			disabled={atLimit || !auth.user?.did}
			class="text-xs px-3 py-1.5 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
			title={!auth.user?.did ? 'Create a DID first' : atLimit ? 'Free tier limit: 1 hosted contract' : ''}
		>
			+ deploy
		</button>
	</div>

	{#if atLimit}
		<p class="text-xs text-[--color-yellow] mb-4">
			Free tier: 1 hosted contract. Upgrade to Pro for multiple contracts.
		</p>
	{/if}

	{#if !auth.user?.did}
		<div class="border border-[--color-border] rounded-lg p-8 text-center">
			<p class="text-xs text-[--color-text-muted] mb-2">No DID found.</p>
			<p class="text-xs text-[--color-text-muted] mb-4">
				You need a DID before you can deploy contracts.
			</p>
			<a href="/dashboard" class="text-xs text-[--color-accent] hover:underline">
				Create a DID on the dashboard →
			</a>
		</div>
	{:else if error}
		<p class="text-xs text-[--color-red] mb-4">{error}</p>
	{:else if loading}
		<p class="text-xs text-[--color-text-muted]">loading…</p>
	{:else if contracts.length === 0}
		<div class="border border-[--color-border] rounded-lg p-8 text-center">
			<p class="text-xs text-[--color-text-muted] mb-2">No deployed contracts.</p>
			<p class="text-xs text-[--color-text-muted] leading-relaxed">
				Upload a WASM binary and we'll host the execution environment for you.
				No server required.
			</p>
			<button
				onclick={() => { showDeploy = true; }}
				class="mt-4 text-xs text-[--color-accent] hover:underline"
			>
				Deploy your first contract →
			</button>
		</div>
	{:else}
		<div class="space-y-3">
			{#each contracts as c}
				<div class="border border-[--color-border] rounded-lg p-4">
					<div class="flex items-start justify-between gap-4">
						<div class="flex-1 min-w-0">
							<p class="text-xs font-mono text-[--color-text-dim] truncate mb-2" title={c.contract_id}>
								{c.contract_id}
							</p>
							<div class="flex items-center gap-4 text-[10px] text-[--color-text-muted]">
								{#if !c.deployed_at && pendingContractId === c.id}
									<span class="text-[--color-yellow]">● deploying…</span>
								{:else}
									<span>deployed {fmtDate(c.deployed_at)}</span>
								{/if}
								<span>{c.execution_count} executions</span>
							</div>
							{#if c.current_state && Object.keys(c.current_state).length > 0}
								<details class="mt-2">
									<summary class="text-[10px] text-[--color-text-muted] cursor-pointer hover:text-[--color-text]">
										current state
									</summary>
									<pre class="text-[10px] text-[--color-text-dim] mt-1 bg-[--color-bg-surface] p-2 rounded overflow-x-auto">{fmtJson(c.current_state)}</pre>
								</details>
							{/if}
						</div>
						<button
							onclick={() => openExecutions(c)}
							class="text-[10px] text-[--color-text-muted] hover:text-[--color-text] transition-colors shrink-0"
						>
							executions
						</button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Deploy modal -->
<Modal bind:open={showDeploy} title="deploy smart contract">
	<div class="space-y-4">
		<div>
			<label class="block text-[10px] text-[--color-text-muted] mb-1.5 uppercase tracking-widest">
				WASM binary (.wasm)
			</label>
			<input
				type="file"
				accept=".wasm"
				onchange={(e) => { wasmFile = (e.target as HTMLInputElement).files?.[0] ?? null; }}
				class="w-full text-xs"
			/>
		</div>
		<div>
			<label class="block text-[10px] text-[--color-text-muted] mb-1.5 uppercase tracking-widest">
				Source file (.rs)
			</label>
			<input
				type="file"
				accept=".rs"
				onchange={(e) => { rsFile = (e.target as HTMLInputElement).files?.[0] ?? null; }}
				class="w-full text-xs"
			/>
		</div>
		<div>
			<label class="block text-[10px] text-[--color-text-muted] mb-1.5 uppercase tracking-widest">
				Initial state (JSON)
			</label>
			<textarea
				bind:value={initialState}
				rows="3"
				class="w-full font-mono text-xs"
				placeholder={'{}' }
			></textarea>
		</div>
		{#if deployError}
			<p class="text-xs text-[--color-red]">{deployError}</p>
		{/if}
		<p class="text-[10px] text-[--color-text-muted] leading-relaxed">
			We'll generate the contract on the Rubix network, register our callback server,
			and deploy it. Execution logs and state history will appear here.
		</p>
		<button
			onclick={handleDeploy}
			disabled={deploying || !wasmFile || !rsFile}
			class="w-full text-xs py-2 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
		>
			{deploying ? 'deploying…' : 'deploy contract'}
		</button>
	</div>
</Modal>

<!-- Signing modal (non-custodial deploy) -->
<Modal bind:open={showSign} title="sign to deploy">
	<div class="space-y-4">
		<p class="text-xs text-[--color-text-muted] leading-relaxed">
			Your wallet signature is required to complete the deployment.
			Enter your PIN to unlock and sign.
		</p>
		<div>
			<label class="block text-[10px] text-[--color-text-muted] mb-1.5 uppercase tracking-widest">
				Wallet PIN
			</label>
			<input
				type="password"
				bind:value={walletPin}
				placeholder="Enter PIN"
				class="w-full text-xs border border-[--color-border] rounded px-2 py-1.5 bg-[--color-bg-surface] text-[--color-text] focus:outline-none focus:border-[--color-accent]"
			/>
		</div>
		{#if signError}
			<p class="text-xs text-[--color-red]">{signError}</p>
		{/if}
		<button
			onclick={handleSign}
			disabled={signing || !walletPin}
			class="w-full text-xs py-2 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
		>
			{signing ? 'signing…' : 'sign & deploy'}
		</button>
	</div>
</Modal>

<!-- Executions modal -->
<Modal bind:open={showExecutions} title={execModalTitle}>
	{#if loadingExec}
		<p class="text-xs text-[--color-text-muted]">loading…</p>
	{:else if execError}
		<p class="text-xs text-[--color-red]">{execError}</p>
	{:else if executions.length === 0}
		<p class="text-xs text-[--color-text-muted]">No executions yet.</p>
	{:else}
		<div class="space-y-3 max-h-96 overflow-y-auto">
			{#each executions as ex}
				<div class="border border-[--color-border] rounded p-3 text-xs">
					<div class="flex items-center justify-between mb-2">
						<span class="{ex.success ? 'text-[--color-green]' : 'text-[--color-red]'}">
							{ex.success ? '✓ success' : '✗ failed'}
						</span>
						<span class="text-[--color-text-muted]">{fmtDate(ex.executed_at)}</span>
					</div>
					{#if ex.error}
						<p class="text-[--color-red] text-[10px] mb-2">{ex.error}</p>
					{/if}
					<details class="text-[10px] text-[--color-text-muted]">
						<summary class="cursor-pointer hover:text-[--color-text]">input / output</summary>
						<div class="mt-2 space-y-2">
							<div>
								<p class="text-[--color-text-muted] mb-1">input</p>
								<pre class="bg-[--color-bg-surface] p-2 rounded overflow-x-auto">{fmtJson(ex.input)}</pre>
							</div>
							<div>
								<p class="text-[--color-text-muted] mb-1">output</p>
								<pre class="bg-[--color-bg-surface] p-2 rounded overflow-x-auto">{fmtJson(ex.output)}</pre>
							</div>
						</div>
					</details>
				</div>
			{/each}
		</div>
	{/if}
</Modal>
