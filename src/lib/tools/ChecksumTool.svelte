<script lang="ts">
	import ToolCard from '$lib/components/ToolCard.svelte';
	import { addHistoryEntry } from '$lib/db';
	import { workspace } from '$lib/stores.svelte';
	import { ethers } from 'ethers';
	import { onMount } from 'svelte';

	let input = $state('');
	let checksummed = $state('');
	let isValid = $state<boolean | null>(null);
	let error = $state('');

	async function check() {
		error = '';
		checksummed = '';
		isValid = null;
		if (!input.trim()) return;

		try {
			const addr = input.trim();
			if (!/^0x[0-9a-fA-F]{40}$/.test(addr)) {
				error = 'Not a valid 20-byte hex address';
				return;
			}
			checksummed = ethers.getAddress(addr);
			isValid = addr === checksummed;
			await addHistoryEntry('checksum-tool', { address: addr }, { checksummed, valid: isValid });
			await workspace.refreshHistory();
		} catch {
			error = 'Invalid address';
		}
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
	}

	onMount(() => {
		if (workspace.sendToInput?.tool === 'checksum-tool') {
			input = workspace.sendToInput.value;
			workspace.sendToInput = null;
			check();
		}
	});
</script>

<ToolCard title="EIP-55 Checksum Address">
	<input
		type="text"
		bind:value={input}
		placeholder="0x..."
		class="w-full font-mono text-xs"
		oninput={check}
	/>

	{#if error}
		<p class="text-xs text-red">{error}</p>
	{/if}

	{#if checksummed}
		<div class="space-y-2">
			<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
				<div class="min-w-0 flex-1">
					<span class="text-[10px] text-text-muted uppercase">Checksummed</span>
					<p class="text-xs text-text font-mono break-all">{checksummed}</p>
				</div>
				<button class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text ml-2 shrink-0" onclick={() => copy(checksummed)}>cp</button>
			</div>
			<div class="bg-bg rounded px-3 py-2 border border-border">
				<span class="text-[10px] text-text-muted uppercase">Valid checksum?</span>
				<p class="text-xs {isValid ? 'text-green' : 'text-yellow'}">{isValid ? 'Yes — already checksummed' : 'No — input was not checksummed'}</p>
			</div>
		</div>
	{/if}
</ToolCard>
