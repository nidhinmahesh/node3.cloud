<script lang="ts">
	import ToolCard from '$lib/components/ToolCard.svelte';
	import { addHistoryEntry } from '$lib/db';
	import { workspace } from '$lib/stores.svelte';
	import { onMount } from 'svelte';

	let input = $state('');
	let mode = $state<'epoch-to-date' | 'date-to-epoch'>('epoch-to-date');
	let result = $state('');
	let relativeTime = $state('');
	let error = $state('');

	const now = $derived(Math.floor(Date.now() / 1000));

	async function convert() {
		error = '';
		result = '';
		relativeTime = '';
		if (!input.trim()) return;

		try {
			if (mode === 'epoch-to-date') {
				let ts = Number(input.trim());
				if (ts > 1e12) ts = Math.floor(ts / 1000);
				const date = new Date(ts * 1000);
				if (isNaN(date.getTime())) throw new Error();
				result = date.toISOString().replace('T', ' ').replace('.000Z', ' UTC');
				const diff = ts - now;
				relativeTime = formatRelative(diff);
			} else {
				const date = new Date(input.trim());
				if (isNaN(date.getTime())) throw new Error();
				const epoch = Math.floor(date.getTime() / 1000);
				result = epoch.toString();
				const diff = epoch - now;
				relativeTime = formatRelative(diff);
			}
			await addHistoryEntry('epoch-converter', { value: input, mode }, { result, relative: relativeTime });
			await workspace.refreshHistory();
		} catch {
			error = mode === 'epoch-to-date' ? 'Invalid timestamp' : 'Invalid date string';
		}
	}

	function formatRelative(diffSec: number): string {
		const abs = Math.abs(diffSec);
		const suffix = diffSec < 0 ? 'ago' : 'from now';
		if (abs < 60) return `${abs}s ${suffix}`;
		if (abs < 3600) return `${Math.floor(abs / 60)}m ${suffix}`;
		if (abs < 86400) return `${Math.floor(abs / 3600)}h ${suffix}`;
		return `${Math.floor(abs / 86400)}d ${suffix}`;
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
	}

	function setNow() {
		input = now.toString();
		convert();
	}

	onMount(() => {
		if (workspace.sendToInput?.tool === 'epoch-converter') {
			input = workspace.sendToInput.value;
			workspace.sendToInput = null;
			convert();
		}
	});
</script>

<ToolCard title="Epoch / Date Converter">
	<div class="flex gap-2">
		<select bind:value={mode} onchange={convert} class="bg-bg border border-border text-text text-xs rounded px-2 py-1">
			<option value="epoch-to-date">Epoch → Date</option>
			<option value="date-to-epoch">Date → Epoch</option>
		</select>
		<button
			class="text-[10px] px-2 py-1 rounded border border-border text-text-dim hover:text-text"
			onclick={setNow}
		>now</button>
	</div>

	<input
		type="text"
		bind:value={input}
		placeholder={mode === 'epoch-to-date' ? 'Unix timestamp (e.g. 1700000000)' : 'Date (e.g. 2024-01-15T12:00:00Z)'}
		class="w-full"
		oninput={convert}
	/>

	{#if error}
		<p class="text-xs text-red">{error}</p>
	{/if}

	{#if result}
		<div class="space-y-2">
			<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
				<div>
					<span class="text-[10px] text-text-muted uppercase">{mode === 'epoch-to-date' ? 'Date' : 'Epoch'}</span>
					<p class="text-xs text-text font-mono">{result}</p>
				</div>
				<button class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text" onclick={() => copy(result)}>cp</button>
			</div>
			{#if relativeTime}
				<div class="bg-bg rounded px-3 py-2 border border-border">
					<span class="text-[10px] text-text-muted uppercase">Relative</span>
					<p class="text-xs text-text-dim">{relativeTime}</p>
				</div>
			{/if}
			<div class="bg-bg rounded px-3 py-2 border border-border">
				<span class="text-[10px] text-text-muted uppercase">Current epoch</span>
				<p class="text-xs text-text-dim font-mono">{now}</p>
			</div>
		</div>
	{/if}
</ToolCard>
