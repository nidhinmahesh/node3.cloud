<script lang="ts">
	import ToolCard from '$lib/components/ToolCard.svelte';
	import { addHistoryEntry } from '$lib/db';
	import { workspace } from '$lib/stores.svelte';
	import { onMount } from 'svelte';

	let input = $state('');
	let inputType = $state<'hex' | 'dec' | 'bin'>('hex');
	let results = $state<{ hex: string; dec: string; bin: string; bytes: string } | null>(null);
	let error = $state('');

	async function convert() {
		error = '';
		results = null;
		if (!input.trim()) return;

		try {
			let value: bigint;
			const cleaned = input.trim();

			if (inputType === 'hex') {
				const h = cleaned.startsWith('0x') ? cleaned.slice(2) : cleaned;
				value = BigInt('0x' + h);
			} else if (inputType === 'dec') {
				value = BigInt(cleaned);
			} else {
				const b = cleaned.startsWith('0b') ? cleaned.slice(2) : cleaned;
				value = BigInt('0b' + b);
			}

			const hex = '0x' + value.toString(16);
			const dec = value.toString(10);
			const bin = value.toString(2);
			const hexNoPre = value.toString(16).padStart(Math.ceil(value.toString(16).length / 2) * 2, '0');
			const bytes = hexNoPre.match(/.{2}/g)?.join(' ') ?? '';

			results = { hex, dec, bin, bytes };
			await addHistoryEntry('hex-converter', { value: cleaned, type: inputType }, { hex, dec, bin });
			await workspace.refreshHistory();
		} catch {
			error = 'Invalid input';
		}
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
	}

	onMount(() => {
		if (workspace.sendToInput?.tool === 'hex-converter') {
			input = workspace.sendToInput.value;
			inputType = /^0x/.test(workspace.sendToInput.value) ? 'hex' : 'dec';
			workspace.sendToInput = null;
			convert();
		}
	});
</script>

<ToolCard title="Hex / Decimal / Binary Converter">
	<div class="flex gap-2">
		<input
			type="text"
			bind:value={input}
			placeholder="0x1a2b or 6699 or 1101..."
			class="flex-1"
			oninput={convert}
		/>
		<select bind:value={inputType} onchange={convert} class="w-20 bg-bg border border-border text-text text-xs rounded px-2 py-1">
			<option value="hex">Hex</option>
			<option value="dec">Dec</option>
			<option value="bin">Bin</option>
		</select>
	</div>

	{#if error}
		<p class="text-xs text-red">{error}</p>
	{/if}

	{#if results}
		<div class="space-y-2">
			{#each [['Hex', results.hex], ['Decimal', results.dec], ['Binary', results.bin], ['Bytes', results.bytes]] as [label, value]}
				<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
					<div class="min-w-0 flex-1">
						<span class="text-[10px] text-text-muted uppercase">{label}</span>
						<p class="text-xs text-text font-mono break-all">{value}</p>
					</div>
					<button
						class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text ml-2 shrink-0"
						onclick={() => copy(value)}
					>cp</button>
				</div>
			{/each}
		</div>
	{/if}
</ToolCard>
