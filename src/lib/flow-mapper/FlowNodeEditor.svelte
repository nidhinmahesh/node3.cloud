<script lang="ts">
	import type { FlowNode } from './types';

	interface Props {
		node: FlowNode;
		zoom: number;
		panX: number;
		panY: number;
		onchange: (node: FlowNode) => void;
		ondelete: () => void;
		onclose: () => void;
	}

	let { node, zoom, panX, panY, onchange, ondelete, onclose }: Props = $props();

	let address = $state('');
	let label = $state('');

	$effect(() => {
		address = node.address;
		label = node.label;
	});

	function commit() {
		onchange({ ...node, address, label });
	}

	let screenX = $derived(Math.max(8, Math.min(node.x * zoom + panX + 190, (typeof window !== 'undefined' ? window.innerWidth : 800) - 250)));
	let screenY = $derived(Math.max(48, Math.min(node.y * zoom + panY, (typeof window !== 'undefined' ? window.innerHeight : 600) - 200)));
</script>

<div
	class="fixed z-50 bg-bg-surface border border-border rounded shadow-lg p-3 w-[calc(100vw-1rem)] max-w-60"
	style="left:{screenX}px;top:{screenY}px;"
>
	<div class="flex items-center justify-between mb-2">
		<span class="text-[10px] text-text-dim uppercase tracking-wider">Edit Node</span>
		<button class="text-text-muted hover:text-text text-xs" onclick={onclose}>&times;</button>
	</div>
	<label class="block mb-2">
		<span class="text-[10px] text-text-dim">Address</span>
		<input
			class="w-full mt-0.5 px-2 py-1 text-xs font-mono bg-bg border border-border rounded text-text focus:border-accent outline-none"
			bind:value={address}
			oninput={commit}
			placeholder="0x..."
		/>
	</label>
	<label class="block mb-3">
		<span class="text-[10px] text-text-dim">Label</span>
		<input
			class="w-full mt-0.5 px-2 py-1 text-xs bg-bg border border-border rounded text-text focus:border-accent outline-none"
			bind:value={label}
			oninput={commit}
			placeholder="e.g. Uniswap Router"
		/>
	</label>
	<button
		class="text-[10px] px-2 py-1 rounded border border-red-800 text-red-400 hover:bg-red-900/30 transition-colors"
		onclick={ondelete}
	>
		Delete Node
	</button>
</div>
