<script lang="ts">
	import ToolCard from '$lib/components/ToolCard.svelte';
	import { addHistoryEntry } from '$lib/db';
	import { workspace } from '$lib/stores.svelte';
	import { onMount } from 'svelte';

	let input = $state('');
	let inputUnit = $state<'wei' | 'gwei' | 'eth'>('wei');
	let results = $state<{ wei: string; gwei: string; eth: string } | null>(null);
	let error = $state('');

	async function convert() {
		error = '';
		results = null;
		if (!input.trim()) return;

		try {
			const val = input.trim();
			let weiBig: bigint;

			if (inputUnit === 'wei') {
				weiBig = BigInt(val);
			} else if (inputUnit === 'gwei') {
				const parts = val.split('.');
				const whole = BigInt(parts[0]) * 1000000000n;
				if (parts[1]) {
					const frac = parts[1].padEnd(9, '0').slice(0, 9);
					weiBig = whole + BigInt(frac);
				} else {
					weiBig = whole;
				}
			} else {
				const parts = val.split('.');
				const whole = BigInt(parts[0]) * 1000000000000000000n;
				if (parts[1]) {
					const frac = parts[1].padEnd(18, '0').slice(0, 18);
					weiBig = whole + BigInt(frac);
				} else {
					weiBig = whole;
				}
			}

			const weiStr = weiBig.toString();
			const gweiStr = formatDecimal(weiBig, 9);
			const ethStr = formatDecimal(weiBig, 18);

			results = { wei: weiStr, gwei: gweiStr, eth: ethStr };
			await addHistoryEntry('unit-converter', { value: val, unit: inputUnit }, { wei: weiStr, gwei: gweiStr, eth: ethStr });
			await workspace.refreshHistory();
		} catch {
			error = 'Invalid number';
		}
	}

	function formatDecimal(wei: bigint, decimals: number): string {
		const divisor = 10n ** BigInt(decimals);
		const whole = wei / divisor;
		const remainder = wei % divisor;
		if (remainder === 0n) return whole.toString();
		const fracStr = remainder.toString().padStart(decimals, '0').replace(/0+$/, '');
		return `${whole}.${fracStr}`;
	}

	function formatWithCommas(s: string): string {
		const [whole, frac] = s.split('.');
		const formatted = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
		return frac ? `${formatted}.${frac}` : formatted;
	}

	async function copy(text: string) {
		await navigator.clipboard.writeText(text);
	}

	function sendToHex(value: string) {
		workspace.sendTo('hex-converter', value);
	}

	onMount(() => {
		if (workspace.sendToInput?.tool === 'unit-converter') {
			input = workspace.sendToInput.value;
			workspace.sendToInput = null;
			convert();
		}
	});
</script>

<ToolCard title="Wei / Gwei / ETH Converter">
	<div class="flex gap-2">
		<input
			type="text"
			bind:value={input}
			placeholder="Enter value..."
			class="flex-1"
			oninput={convert}
		/>
		<select bind:value={inputUnit} onchange={convert} class="w-24 bg-bg border border-border text-text text-xs rounded px-2 py-1">
			<option value="wei">Wei</option>
			<option value="gwei">Gwei</option>
			<option value="eth">ETH</option>
		</select>
	</div>

	{#if error}
		<p class="text-xs text-red">{error}</p>
	{/if}

	{#if results}
		<div class="space-y-2">
			{#each [['Wei', results.wei], ['Gwei', results.gwei], ['ETH', results.eth]] as [label, value]}
				<div class="flex items-center justify-between bg-bg rounded px-3 py-2 border border-border">
					<div>
						<span class="text-[10px] text-text-muted uppercase">{label}</span>
						<p class="text-xs text-text font-mono">{formatWithCommas(value)}</p>
					</div>
					<div class="flex gap-1">
						<button
							class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-text"
							onclick={() => copy(value)}
						>cp</button>
						{#if label === 'Wei'}
							<button
								class="text-[10px] px-1.5 py-0.5 rounded border border-border text-text-dim hover:text-accent"
								onclick={() => sendToHex(value)}
							>→hex</button>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</ToolCard>
