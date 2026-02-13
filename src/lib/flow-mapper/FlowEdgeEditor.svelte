<script lang="ts">
	import type { FlowEdge, FlowNode } from './types';

	interface Props {
		edge: FlowEdge;
		nodes: FlowNode[];
		zoom: number;
		panX: number;
		panY: number;
		onchange: (edge: FlowEdge) => void;
		ondelete: () => void;
		onclose: () => void;
	}

	let { edge, nodes, zoom, panX, panY, onchange, ondelete, onclose }: Props = $props();

	let label = $state('');

	$effect(() => {
		label = edge.label;
	});

	function commit() {
		onchange({ ...edge, label });
	}

	let screenPos = $derived.by(() => {
		const src = nodes.find((n) => n.id === edge.from);
		const dst = nodes.find((n) => n.id === edge.to);
		if (!src || !dst) return { x: 100, y: 100 };
		const mx = ((src.x + dst.x) / 2) * zoom + panX;
		const my = ((src.y + dst.y) / 2) * zoom + panY;
		return { x: mx + 20, y: my - 40 };
	});
</script>

<div
	class="fixed z-50 bg-bg-surface border border-border rounded shadow-lg p-3 w-56"
	style="left:{screenPos.x}px;top:{screenPos.y}px;"
>
	<div class="flex items-center justify-between mb-2">
		<span class="text-[10px] text-text-dim uppercase tracking-wider">Edit Edge</span>
		<button class="text-text-muted hover:text-text text-xs" onclick={onclose}>&times;</button>
	</div>
	<label class="block mb-3">
		<span class="text-[10px] text-text-dim">Label</span>
		<input
			class="w-full mt-0.5 px-2 py-1 text-xs font-mono bg-bg border border-border rounded text-text focus:border-accent outline-none"
			bind:value={label}
			oninput={commit}
			placeholder="e.g. 1.5 ETH, tx hash..."
		/>
	</label>
	<button
		class="text-[10px] px-2 py-1 rounded border border-red-800 text-red-400 hover:bg-red-900/30 transition-colors"
		onclick={ondelete}
	>
		Delete Edge
	</button>
</div>
