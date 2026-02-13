<script lang="ts">
	import type { FlowEdge, FlowNode } from './types';
	import { portOut, portIn, bezierPath } from './utils';

	interface Props {
		edge: FlowEdge;
		nodes: FlowNode[];
		selected: boolean;
		onclick: (e: MouseEvent) => void;
	}

	let { edge, nodes, selected, onclick }: Props = $props();

	let path = $derived.by(() => {
		const src = nodes.find((n) => n.id === edge.from);
		const dst = nodes.find((n) => n.id === edge.to);
		if (!src || !dst) return '';
		const p1 = portOut(src.x, src.y);
		const p2 = portIn(dst.x, dst.y);
		return bezierPath(p1.x, p1.y, p2.x, p2.y);
	});

	let midPoint = $derived.by(() => {
		const src = nodes.find((n) => n.id === edge.from);
		const dst = nodes.find((n) => n.id === edge.to);
		if (!src || !dst) return { x: 0, y: 0 };
		const p1 = portOut(src.x, src.y);
		const p2 = portIn(dst.x, dst.y);
		return { x: (p1.x + p2.x) / 2, y: (p1.y + p2.y) / 2 };
	});
</script>

{#if path}
	<!-- hit area -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<path
		d={path}
		fill="none"
		stroke="transparent"
		stroke-width="12"
		class="cursor-pointer"
		onclick={onclick}
	/>
	<path
		d={path}
		fill="none"
		class={selected ? 'stroke-accent' : 'stroke-border-bright'}
		stroke-width="1.5"
		pointer-events="none"
	/>
	{#if edge.label}
		<text
			x={midPoint.x}
			y={midPoint.y - 6}
			text-anchor="middle"
			class="fill-text-dim text-[10px] pointer-events-none"
		>
			{edge.label}
		</text>
	{/if}
{/if}
