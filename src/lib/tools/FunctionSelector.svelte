<script lang="ts">
	import ToolCard from '$lib/components/ToolCard.svelte';
	import { addHistoryEntry } from '$lib/db';
	import { workspace } from '$lib/stores.svelte';
	import { ethers } from 'ethers';
	import { onMount } from 'svelte';

	let input = $state('');
	let selector = $state('');
	let fullHash = $state('');
	let error = $state('');

	async function compute() {
		error = '';
		selector = '';
		fullHash = '';
		if (!input.trim()) return;

		try {
			const sig = input.trim();
			if (!sig.includes('(')) {
				error = 'Enter a function signature like "transfer(address,uint256)"';
				return;
			}
			const hash = ethers.keccak256(ethers.toUtf8Bytes(sig));
			fullHash = hash;
			selector = hash.slice(0, 10);
			await addHistoryEntry('function-selector', { signature: sig }, { selector, hash });
			await workspace.refreshHistory();
		} catch {
			error = 'Invalid function signature';
		}
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
	}

	onMount(() => {
		if (workspace.sendToInput?.tool === 'function-selector') {
			input = workspace.sendToInput.value;
			workspace.sendToInput = null;
			compute();
		}
	});
</script>

<ToolCard title="Function Selector (4-byte)">
	<input
		type="text"
		bind:value={input}
		placeholder='transfer(address,uint256)'
		class="w-full"
		oninput={compute}
	/>

	{#if error}
		<p class="text-xs text-red">{error}</p>
	{/if}

	{#if selector}
		<div class="space-y-2">
			<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
				<div>
					<span class="text-[10px] text-text-muted uppercase">Selector (4 bytes)</span>
					<p class="text-sm text-accent font-mono font-semibold">{selector}</p>
				</div>
				<button class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text" onclick={() => copy(selector)}>cp</button>
			</div>
			<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
				<div class="min-w-0 flex-1">
					<span class="text-[10px] text-text-muted uppercase">Full Keccak256</span>
					<p class="text-xs text-text-dim font-mono break-all">{fullHash}</p>
				</div>
				<button class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text ml-2 shrink-0" onclick={() => copy(fullHash)}>cp</button>
			</div>
		</div>
	{/if}
</ToolCard>
