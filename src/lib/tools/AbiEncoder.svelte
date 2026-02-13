<script lang="ts">
	import ToolCard from '$lib/components/ToolCard.svelte';
	import { addHistoryEntry } from '$lib/db';
	import { workspace } from '$lib/stores.svelte';
	import { ethers } from 'ethers';
	import { onMount } from 'svelte';

	let mode = $state<'encode' | 'decode'>('encode');
	let signature = $state('');
	let params = $state('');
	let calldata = $state('');
	let result = $state('');
	let decoded = $state<{ name: string; values: string[] } | null>(null);
	let error = $state('');

	async function encode() {
		error = '';
		result = '';
		if (!signature.trim() || !params.trim()) return;

		try {
			const iface = new ethers.Interface([`function ${signature.trim()}`]);
			const funcName = signature.trim().split('(')[0];
			const args = JSON.parse(`[${params}]`);
			result = iface.encodeFunctionData(funcName, args);
			await addHistoryEntry('abi-encoder', { signature, params, mode: 'encode' }, { calldata: result });
			await workspace.refreshHistory();
		} catch (e) {
			error = `Encode failed: ${e instanceof Error ? e.message : 'unknown error'}`;
		}
	}

	async function decode() {
		error = '';
		decoded = null;
		if (!signature.trim() || !calldata.trim()) return;

		try {
			const iface = new ethers.Interface([`function ${signature.trim()}`]);
			const funcName = signature.trim().split('(')[0];
			const fragment = iface.getFunction(funcName);
			if (!fragment) throw new Error('Function not found in ABI');
			const res = iface.decodeFunctionData(fragment, calldata.trim());
			const values = fragment.inputs.map((inp, i) => `${inp.name || 'arg' + i} (${inp.type}): ${res[i].toString()}`);
			decoded = { name: funcName, values };
			await addHistoryEntry('abi-encoder', { signature, calldata, mode: 'decode' }, { function: funcName, params: values });
			await workspace.refreshHistory();
		} catch (e) {
			error = `Decode failed: ${e instanceof Error ? e.message : 'unknown error'}`;
		}
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
	}

	onMount(() => {
		if (workspace.sendToInput?.tool === 'abi-encoder') {
			calldata = workspace.sendToInput.value;
			mode = 'decode';
			workspace.sendToInput = null;
		}
	});
</script>

<ToolCard title="ABI Encoder / Decoder">
	<div class="flex gap-2">
		<select bind:value={mode} class="bg-bg border border-border text-text text-xs rounded px-2 py-1">
			<option value="encode">Encode</option>
			<option value="decode">Decode</option>
		</select>
	</div>

	<input
		type="text"
		bind:value={signature}
		placeholder='transfer(address to, uint256 amount)'
		class="w-full"
	/>

	{#if mode === 'encode'}
		<div>
			<span class="text-[10px] text-text-muted uppercase block mb-1">Parameters (comma-separated, JSON-style)</span>
			<textarea
				bind:value={params}
				placeholder='"0xAbC...123", 1000000'
				rows="2"
				class="w-full"
			></textarea>
		</div>
		<button
			class="text-xs px-3 py-1.5 rounded bg-accent text-bg font-semibold hover:bg-accent-hover transition-colors"
			onclick={encode}
		>Encode</button>
	{:else}
		<div>
			<span class="text-[10px] text-text-muted uppercase block mb-1">Calldata</span>
			<textarea
				bind:value={calldata}
				placeholder='0xa9059cbb...'
				rows="3"
				class="w-full"
			></textarea>
		</div>
		<button
			class="text-xs px-3 py-1.5 rounded bg-accent text-bg font-semibold hover:bg-accent-hover transition-colors"
			onclick={decode}
		>Decode</button>
	{/if}

	{#if error}
		<p class="text-xs text-red">{error}</p>
	{/if}

	{#if result}
		<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
			<div class="min-w-0 flex-1">
				<span class="text-[10px] text-text-muted uppercase">Encoded Calldata</span>
				<p class="text-xs text-text font-mono break-all">{result}</p>
			</div>
			<button class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text ml-2 shrink-0" onclick={() => copy(result)}>cp</button>
		</div>
	{/if}

	{#if decoded}
		<div class="bg-bg rounded px-3 py-2 border border-border space-y-1">
			<span class="text-[10px] text-text-muted uppercase">Decoded: {decoded.name}()</span>
			{#each decoded.values as val}
				<p class="text-xs text-text font-mono">{val}</p>
			{/each}
		</div>
	{/if}
</ToolCard>
