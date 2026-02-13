<script lang="ts">
	import ToolCard from '$lib/components/ToolCard.svelte';
	import { addHistoryEntry } from '$lib/db';
	import { workspace } from '$lib/stores.svelte';
	import { ethers } from 'ethers';
	import { onMount } from 'svelte';

	let input = $state('');
	let inputMode = $state<'utf8' | 'hex'>('utf8');
	let hash = $state('');
	let error = $state('');

	async function compute() {
		error = '';
		hash = '';
		if (!input.trim()) return;

		try {
			if (inputMode === 'utf8') {
				hash = ethers.keccak256(ethers.toUtf8Bytes(input));
			} else {
				const hex = input.trim().startsWith('0x') ? input.trim() : '0x' + input.trim();
				if (!/^0x[0-9a-fA-F]*$/.test(hex) || hex.length % 2 !== 0) {
					error = 'Invalid hex (must be even length)';
					return;
				}
				hash = ethers.keccak256(hex);
			}
			await addHistoryEntry('keccak256', { value: input, mode: inputMode }, { hash });
			await workspace.refreshHistory();
		} catch {
			error = 'Failed to hash';
		}
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
	}

	function sendToHex() {
		workspace.sendTo('hex-converter', hash);
	}

	onMount(() => {
		if (workspace.sendToInput?.tool === 'keccak256') {
			input = workspace.sendToInput.value;
			workspace.sendToInput = null;
			compute();
		}
	});
</script>

<ToolCard title="Keccak256 Hash">
	<div class="flex gap-2">
		<select bind:value={inputMode} onchange={compute} class="bg-bg border border-border text-text text-xs rounded px-2 py-1">
			<option value="utf8">UTF-8</option>
			<option value="hex">Hex</option>
		</select>
	</div>

	<textarea
		bind:value={input}
		placeholder={inputMode === 'utf8' ? 'Text to hash...' : '0xdeadbeef...'}
		rows="2"
		class="w-full"
		oninput={compute}
	></textarea>

	{#if error}
		<p class="text-xs text-red">{error}</p>
	{/if}

	{#if hash}
		<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
			<div class="min-w-0 flex-1">
				<span class="text-[10px] text-text-muted uppercase">Keccak256</span>
				<p class="text-xs text-text font-mono break-all">{hash}</p>
			</div>
			<div class="flex gap-1 ml-2 shrink-0">
				<button class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text" onclick={() => copy(hash)}>cp</button>
				<button class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-accent" onclick={sendToHex}>→hex</button>
			</div>
		</div>
	{/if}
</ToolCard>
