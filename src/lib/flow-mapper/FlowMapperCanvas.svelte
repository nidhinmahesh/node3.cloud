<script lang="ts">
	import { onMount } from 'svelte';
	import type { FlowNode, FlowEdge, FlowGraph } from './types';
	import { nodeId, edgeId, NODE_W, NODE_H, screenToWorld, portOut, portIn, bezierPath, MIN_ZOOM, MAX_ZOOM } from './utils';
	import { saveGraph, loadGraph, listGraphs, deleteGraph } from './db-helpers';
	import FlowToolbar from './FlowToolbar.svelte';
	import FlowNodeCard from './FlowNodeCard.svelte';
	import FlowEdgePath from './FlowEdgePath.svelte';
	import FlowNodeEditor from './FlowNodeEditor.svelte';
	import FlowEdgeEditor from './FlowEdgeEditor.svelte';

	type Mode = 'idle' | 'panning' | 'dragging-node' | 'connecting';

	let containerEl: HTMLDivElement;

	// graph state
	let graphId = $state<number | undefined>(undefined);
	let graphName = $state('Untitled');
	let nodes = $state<FlowNode[]>([]);
	let edges = $state<FlowEdge[]>([]);
	let panX = $state(0);
	let panY = $state(0);
	let zoom = $state(1);

	// interaction state
	let mode = $state<Mode>('idle');
	let dragNodeId = $state<string | null>(null);
	let dragOffsetX = 0;
	let dragOffsetY = 0;
	let connectFromId = $state<string | null>(null);
	let tempLineEnd = $state<{ x: number; y: number } | null>(null);
	let panStartX = 0;
	let panStartY = 0;
	let panStartPanX = 0;
	let panStartPanY = 0;

	// selection
	let selectedNodeId = $state<string | null>(null);
	let selectedEdgeId = $state<string | null>(null);

	// graph list for toolbar
	let graphList = $state<{ id?: number; name: string; updatedAt: string }[]>([]);

	// debounced save
	let saveTimer: ReturnType<typeof setTimeout> | undefined;

	function scheduleSave() {
		clearTimeout(saveTimer);
		saveTimer = setTimeout(persistGraph, 500);
	}

	async function persistGraph() {
		const graph: FlowGraph = {
			id: graphId,
			name: graphName,
			nodes: $state.snapshot(nodes),
			edges: $state.snapshot(edges),
			viewport: { panX, panY, zoom },
			createdAt: '',
			updatedAt: ''
		};
		const id = await saveGraph(graph);
		if (!graphId) graphId = id;
		await refreshList();
	}

	async function refreshList() {
		graphList = await listGraphs();
	}

	// --- Mouse handlers ---

	function onCanvasMouseDown(e: MouseEvent) {
		if (e.button !== 0) return;
		// clicked on empty canvas → pan
		mode = 'panning';
		panStartX = e.clientX;
		panStartY = e.clientY;
		panStartPanX = panX;
		panStartPanY = panY;
		selectedNodeId = null;
		selectedEdgeId = null;
	}

	function onNodeMouseDown(e: MouseEvent, node: FlowNode) {
		e.stopPropagation();
		mode = 'dragging-node';
		dragNodeId = node.id;
		selectedNodeId = node.id;
		selectedEdgeId = null;
		dragOffsetX = e.clientX / zoom - node.x;
		dragOffsetY = e.clientY / zoom - node.y;
	}

	function onPortMouseDown(e: MouseEvent, node: FlowNode) {
		e.stopPropagation();
		mode = 'connecting';
		connectFromId = node.id;
		const rect = containerEl.getBoundingClientRect();
		tempLineEnd = screenToWorld(e.clientX - rect.left, e.clientY - rect.top, panX, panY, zoom);
	}

	function onMouseMove(e: MouseEvent) {
		if (mode === 'panning') {
			panX = panStartPanX + (e.clientX - panStartX);
			panY = panStartPanY + (e.clientY - panStartY);
		} else if (mode === 'dragging-node' && dragNodeId) {
			const n = nodes.find((n) => n.id === dragNodeId);
			if (n) {
				n.x = e.clientX / zoom - dragOffsetX;
				n.y = e.clientY / zoom - dragOffsetY;
			}
		} else if (mode === 'connecting') {
			const rect = containerEl.getBoundingClientRect();
			tempLineEnd = screenToWorld(e.clientX - rect.left, e.clientY - rect.top, panX, panY, zoom);
		}
	}

	function onMouseUp(e: MouseEvent) {
		if (mode === 'connecting' && connectFromId) {
			// check if released over a node
			const rect = containerEl.getBoundingClientRect();
			const world = screenToWorld(e.clientX - rect.left, e.clientY - rect.top, panX, panY, zoom);
			const target = nodes.find(
				(n) =>
					n.id !== connectFromId &&
					world.x >= n.x &&
					world.x <= n.x + NODE_W &&
					world.y >= n.y &&
					world.y <= n.y + NODE_H
			);
			if (target) {
				// prevent duplicate edges
				const exists = edges.some(
					(e) => e.from === connectFromId && e.to === target.id
				);
				if (!exists) {
					edges.push({ id: edgeId(), from: connectFromId!, to: target.id, label: '' });
					scheduleSave();
				}
			}
		}
		if (mode === 'dragging-node') {
			scheduleSave();
		}
		mode = 'idle';
		dragNodeId = null;
		connectFromId = null;
		tempLineEnd = null;
	}

	function onWheel(e: WheelEvent) {
		e.preventDefault();
		const rect = containerEl.getBoundingClientRect();
		const mx = e.clientX - rect.left;
		const my = e.clientY - rect.top;

		const oldZoom = zoom;
		const factor = e.deltaY < 0 ? 1.1 : 0.9;
		zoom = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, zoom * factor));

		// zoom toward cursor
		panX = mx - (mx - panX) * (zoom / oldZoom);
		panY = my - (my - panY) * (zoom / oldZoom);
	}

	function onCanvasDblClick(e: MouseEvent) {
		const rect = containerEl.getBoundingClientRect();
		const world = screenToWorld(e.clientX - rect.left, e.clientY - rect.top, panX, panY, zoom);
		const newNode: FlowNode = {
			id: nodeId(),
			address: '',
			label: '',
			x: world.x - NODE_W / 2,
			y: world.y - NODE_H / 2
		};
		nodes.push(newNode);
		selectedNodeId = newNode.id;
		selectedEdgeId = null;
		scheduleSave();
	}

	// --- Temp bezier line for connecting ---
	let tempPath = $derived.by(() => {
		if (!connectFromId || !tempLineEnd) return '';
		const src = nodes.find((n) => n.id === connectFromId);
		if (!src) return '';
		const p1 = portOut(src.x, src.y);
		return bezierPath(p1.x, p1.y, tempLineEnd.x, tempLineEnd.y);
	});

	// --- Toolbar actions ---

	function handleAddNode() {
		const cx = (-panX + (containerEl?.clientWidth ?? 600) / 2) / zoom;
		const cy = (-panY + (containerEl?.clientHeight ?? 400) / 2) / zoom;
		const newNode: FlowNode = { id: nodeId(), address: '', label: '', x: cx - NODE_W / 2, y: cy - NODE_H / 2 };
		nodes.push(newNode);
		selectedNodeId = newNode.id;
		selectedEdgeId = null;
		scheduleSave();
	}

	function handleClear() {
		nodes = [];
		edges = [];
		selectedNodeId = null;
		selectedEdgeId = null;
		scheduleSave();
	}

	async function handleNew() {
		graphId = undefined;
		graphName = 'Untitled';
		nodes = [];
		edges = [];
		panX = 0;
		panY = 0;
		zoom = 1;
		selectedNodeId = null;
		selectedEdgeId = null;
	}

	async function handleLoad(id: number) {
		const g = await loadGraph(id);
		if (!g) return;
		graphId = g.id;
		graphName = g.name;
		nodes = g.nodes;
		edges = g.edges;
		panX = g.viewport.panX;
		panY = g.viewport.panY;
		zoom = g.viewport.zoom;
		selectedNodeId = null;
		selectedEdgeId = null;
	}

	async function handleDelete() {
		if (graphId) {
			await deleteGraph(graphId);
			await handleNew();
			await refreshList();
		}
	}

	function handleNameChange(name: string) {
		graphName = name;
		scheduleSave();
	}

	// --- Node/Edge editor callbacks ---

	function onNodeChange(updated: FlowNode) {
		const idx = nodes.findIndex((n) => n.id === updated.id);
		if (idx !== -1) {
			nodes[idx] = updated;
			scheduleSave();
		}
	}

	function onNodeDelete() {
		if (!selectedNodeId) return;
		nodes = nodes.filter((n) => n.id !== selectedNodeId);
		edges = edges.filter((e) => e.from !== selectedNodeId && e.to !== selectedNodeId);
		selectedNodeId = null;
		scheduleSave();
	}

	function onEdgeChange(updated: FlowEdge) {
		const idx = edges.findIndex((e) => e.id === updated.id);
		if (idx !== -1) {
			edges[idx] = updated;
			scheduleSave();
		}
	}

	function onEdgeDelete() {
		if (!selectedEdgeId) return;
		edges = edges.filter((e) => e.id !== selectedEdgeId);
		selectedEdgeId = null;
		scheduleSave();
	}

	function onEdgeClick(e: MouseEvent, edge: FlowEdge) {
		e.stopPropagation();
		selectedEdgeId = edge.id;
		selectedNodeId = null;
	}

	function onNodeDblClick(e: MouseEvent, node: FlowNode) {
		e.stopPropagation();
		selectedNodeId = node.id;
		selectedEdgeId = null;
	}

	// --- Reactive derivations ---
	let selectedNode = $derived(nodes.find((n) => n.id === selectedNodeId) ?? null);
	let selectedEdge = $derived(edges.find((e) => e.id === selectedEdgeId) ?? null);

	onMount(() => {
		refreshList().then(async () => {
			// load most recent graph if any
			if (graphList.length > 0 && graphList[0].id) {
				await handleLoad(graphList[0].id);
			}
		});
	});
</script>

<!-- svelte-ignore a11y_no_static_element_interactions a11y_no_noninteractive_element_interactions -->
<div
	bind:this={containerEl}
	class="relative w-full h-full overflow-hidden bg-bg cursor-default"
	onmousedown={onCanvasMouseDown}
	onmousemove={onMouseMove}
	onmouseup={onMouseUp}
	ondblclick={onCanvasDblClick}
	onwheel={onWheel}
	role="application"
>
	<FlowToolbar
		{graphName}
		{graphList}
		onnamechange={handleNameChange}
		onsave={persistGraph}
		onnew={handleNew}
		onload={handleLoad}
		ondelete={handleDelete}
		onaddnode={handleAddNode}
		onclear={handleClear}
	/>

	<!-- SVG layer for edges -->
	<svg
		class="absolute inset-0 w-full h-full pointer-events-none"
		style="overflow:visible;"
	>
		<g transform="translate({panX},{panY}) scale({zoom})" class="pointer-events-auto">
			{#each edges as edge (edge.id)}
				<FlowEdgePath
					{edge}
					{nodes}
					selected={edge.id === selectedEdgeId}
					onclick={(e) => onEdgeClick(e, edge)}
				/>
			{/each}
			{#if tempPath}
				<path d={tempPath} fill="none" class="stroke-accent" stroke-width="1.5" stroke-dasharray="4 4" pointer-events="none" />
			{/if}
		</g>
	</svg>

	<!-- DOM layer for nodes -->
	<div
		class="absolute inset-0 pointer-events-none"
		style="transform:translate({panX}px,{panY}px) scale({zoom});transform-origin:0 0;"
	>
		<div class="pointer-events-auto">
			{#each nodes as node (node.id)}
				<FlowNodeCard
					{node}
					{zoom}
					selected={node.id === selectedNodeId}
					onmousedown={(e) => onNodeMouseDown(e, node)}
					onportdown={(e) => onPortMouseDown(e, node)}
					ondblclick={(e) => onNodeDblClick(e, node)}
				/>
			{/each}
		</div>
	</div>

	<!-- Editors -->
	{#if selectedNode}
		<FlowNodeEditor
			node={selectedNode}
			{zoom}
			{panX}
			{panY}
			onchange={onNodeChange}
			ondelete={onNodeDelete}
			onclose={() => (selectedNodeId = null)}
		/>
	{/if}
	{#if selectedEdge}
		<FlowEdgeEditor
			edge={selectedEdge}
			{nodes}
			{zoom}
			{panX}
			{panY}
			onchange={onEdgeChange}
			ondelete={onEdgeDelete}
			onclose={() => (selectedEdgeId = null)}
		/>
	{/if}

	<!-- Hint when empty -->
	{#if nodes.length === 0}
		<div class="absolute inset-0 flex items-center justify-center pointer-events-none">
			<span class="text-text-dim text-sm">Double-click to add a node</span>
		</div>
	{/if}
</div>
