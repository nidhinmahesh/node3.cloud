<script lang="ts">
	import ToolCard from '$lib/components/ToolCard.svelte';
	import { addHistoryEntry } from '$lib/db';
	import { workspace } from '$lib/stores.svelte';
	import { onMount } from 'svelte';

	let input = $state('');
	let mode = $state<'encode' | 'decode'>('encode');
	let result = $state('');
	let hexResult = $state('');
	let error = $state('');

	async function process() {
		error = '';
		result = '';
		hexResult = '';
		if (!input.trim()) return;

		try {
			if (mode === 'encode') {
				result = btoa(input);
				hexResult = Array.from(new TextEncoder().encode(input))
					.map((b) => b.toString(16).padStart(2, '0'))
					.join('');
			} else {
				result = atob(input.trim());
				hexResult = Array.from(new TextEncoder().encode(result))
					.map((b) => b.toString(16).padStart(2, '0'))
					.join('');
			}
			await addHistoryEntry('base64-codec', { value: input, mode }, { result, hex: hexResult });
			await workspace.refreshHistory();
		} catch {
			error = mode === 'decode' ? 'Invalid Base64 string' : 'Invalid input';
		}
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
	}

	onMount(() => {
		if (workspace.sendToInput?.tool === 'base64-codec') {
			input = workspace.sendToInput.value;
			workspace.sendToInput = null;
			process();
		}
	});
</script>

<ToolCard title="Base64 / UTF-8 / Hex Codec">
	<div class="flex gap-2">
		<select bind:value={mode} onchange={process} class="w-28 bg-bg border border-border text-text text-xs rounded px-2 py-1">
			<option value="encode">Encode</option>
			<option value="decode">Decode</option>
		</select>
	</div>

	<textarea
		bind:value={input}
		placeholder={mode === 'encode' ? 'Text to encode...' : 'Base64 to decode...'}
		rows="3"
		class="w-full"
		oninput={process}
	></textarea>

	{#if error}
		<p class="text-xs text-red">{error}</p>
	{/if}

	{#if result}
		<div class="space-y-2">
			<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
				<div class="min-w-0 flex-1">
					<span class="text-[10px] text-text-muted uppercase">{mode === 'encode' ? 'Base64' : 'UTF-8'}</span>
					<p class="text-xs text-text font-mono break-all">{result}</p>
				</div>
				<button class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text ml-2 shrink-0" onclick={() => copy(result)}>cp</button>
			</div>
			<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
				<div class="min-w-0 flex-1">
					<span class="text-[10px] text-text-muted uppercase">Hex</span>
					<p class="text-xs text-text font-mono break-all">0x{hexResult}</p>
				</div>
				<button class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text ml-2 shrink-0" onclick={() => copy('0x' + hexResult)}>cp</button>
			</div>
		</div>
	{/if}
</ToolCard>
