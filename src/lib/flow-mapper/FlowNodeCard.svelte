<script lang="ts">
	import type { FlowNode } from './types';
	import { NODE_W, NODE_H, PORT_R } from './utils';

	interface Props {
		node: FlowNode;
		selected: boolean;
		zoom: number;
		onmousedown: (e: MouseEvent) => void;
		onportdown: (e: MouseEvent) => void;
		ondblclick: (e: MouseEvent) => void;
	}

	let { node, selected, zoom, onmousedown, onportdown, ondblclick }: Props = $props();

	function truncate(addr: string): string {
		if (addr.length <= 12) return addr;
		return addr.slice(0, 6) + '...' + addr.slice(-4);
	}

	function handlePortDown(e: MouseEvent) {
		e.stopPropagation();
		onportdown(e);
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="absolute select-none cursor-grab active:cursor-grabbing rounded border {selected
		? 'border-accent bg-bg-surface/95'
		: 'border-border bg-bg-surface/90'} hover:border-border-bright transition-colors"
	style="left:{node.x}px;top:{node.y}px;width:{NODE_W}px;height:{NODE_H}px;"
	onmousedown={onmousedown}
	ondblclick={ondblclick}
>
	<div class="px-2 pt-1.5 overflow-hidden">
		<div class="text-xs text-text font-mono truncate">
			{node.address || 'New Node'}
		</div>
		<div class="text-[10px] text-text-dim truncate">
			{node.label || 'double-click to edit'}
		</div>
	</div>

	<!-- output port -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="absolute bg-accent rounded-full hover:scale-150 transition-transform cursor-crosshair"
		style="right:-{PORT_R}px;top:{NODE_H / 2 - PORT_R}px;width:{PORT_R * 2}px;height:{PORT_R * 2}px;"
		onmousedown={handlePortDown}
	></div>
</div>
