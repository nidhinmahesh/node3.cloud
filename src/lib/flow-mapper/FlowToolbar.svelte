<script lang="ts">
	interface GraphOption {
		id?: number;
		name: string;
		updatedAt: string;
	}

	interface Props {
		graphName: string;
		graphList: GraphOption[];
		onnamechange: (name: string) => void;
		onsave: () => void;
		onnew: () => void;
		onload: (id: number) => void;
		ondelete: () => void;
		onaddnode: () => void;
		onclear: () => void;
	}

	let {
		graphName,
		graphList,
		onnamechange,
		onsave,
		onnew,
		onload,
		ondelete,
		onaddnode,
		onclear
	}: Props = $props();

	let showList = $state(false);
</script>

<div
	class="absolute top-0 left-0 right-0 z-40 flex items-center gap-2 px-3 py-2 bg-bg-surface/80 backdrop-blur border-b border-border"
>
	<input
		class="px-2 py-1 text-xs bg-bg border border-border rounded text-text w-40 focus:border-accent outline-none"
		value={graphName}
		oninput={(e) => onnamechange(e.currentTarget.value)}
		placeholder="Graph name"
	/>

	<button
		class="text-[10px] px-2 py-1 rounded border border-border text-text-muted hover:text-text hover:border-border-bright transition-colors"
		onclick={onsave}>save</button
	>
	<button
		class="text-[10px] px-2 py-1 rounded border border-border text-text-muted hover:text-text hover:border-border-bright transition-colors"
		onclick={onnew}>new</button
	>

	<div class="relative">
		<button
			class="text-[10px] px-2 py-1 rounded border border-border text-text-muted hover:text-text hover:border-border-bright transition-colors"
			onclick={() => (showList = !showList)}>load</button
		>
		{#if showList && graphList.length > 0}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="absolute top-full left-0 mt-1 bg-bg-surface border border-border rounded shadow-lg min-w-[180px] max-h-48 overflow-y-auto"
				onmouseleave={() => (showList = false)}
			>
				{#each graphList as g}
					<button
						class="block w-full text-left px-3 py-1.5 text-xs text-text hover:bg-bg transition-colors"
						onclick={() => {
							if (g.id) onload(g.id);
							showList = false;
						}}
					>
						{g.name}
					</button>
				{/each}
			</div>
		{/if}
	</div>

	<div class="w-px h-4 bg-border mx-1"></div>

	<button
		class="text-[10px] px-2 py-1 rounded border border-border text-text-muted hover:text-text hover:border-border-bright transition-colors"
		onclick={onaddnode}>+ node</button
	>
	<button
		class="text-[10px] px-2 py-1 rounded border border-border text-text-muted hover:text-text hover:border-border-bright transition-colors"
		onclick={onclear}>clear</button
	>

	{#if graphList.length > 0}
		<button
			class="ml-auto text-[10px] px-2 py-1 rounded border border-red-800 text-red-400 hover:bg-red-900/30 transition-colors"
			onclick={ondelete}>delete graph</button
		>
	{/if}
</div>
