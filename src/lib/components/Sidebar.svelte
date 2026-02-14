<script lang="ts">
	import { workspace, tools, categories, type ToolId } from '$lib/stores.svelte';

	function grouped() {
		const map = new Map<string, typeof tools>();
		for (const cat of categories) {
			map.set(cat, tools.filter((t) => t.category === cat));
		}
		return map;
	}

	function selectTool(id: ToolId) {
		workspace.setTool(id);
		workspace.sidebarOpen = false;
	}
</script>

<!-- mobile overlay backdrop -->
{#if workspace.sidebarOpen}
	<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
	<div
		class="fixed inset-0 bg-black/50 z-40 md:hidden"
		onclick={() => (workspace.sidebarOpen = false)}
	></div>
{/if}

<aside class="flex flex-col h-full bg-bg-surface border-r border-border w-56 shrink-0
	fixed z-50 top-0 left-0 transition-transform duration-200 md:static md:translate-x-0
	{workspace.sidebarOpen ? 'translate-x-0' : '-translate-x-full'}">
	<div class="px-4 py-3 border-b border-border flex items-center justify-between">
		<h1 class="text-sm font-semibold text-accent tracking-wide">node3.cloud</h1>
		<button
			class="md:hidden text-text-muted hover:text-text text-sm"
			onclick={() => (workspace.sidebarOpen = false)}
		>&times;</button>
	</div>

	<nav class="flex-1 overflow-y-auto py-2">
		{#each [...grouped()] as [category, items]}
			<div class="px-3 pt-3 pb-1">
				<span class="text-[10px] uppercase tracking-widest text-text-muted">{category}</span>
			</div>
			{#each items as tool}
				<button
					class="w-full text-left px-4 py-1.5 text-xs transition-colors {workspace.activeTool === tool.id
						? 'bg-bg-active text-accent'
						: 'text-text-dim hover:bg-bg-hover hover:text-text'}"
					onclick={() => selectTool(tool.id)}
				>
					{tool.name}
				</button>
			{/each}
		{/each}
	</nav>

	<div class="border-t border-border p-3 space-y-1">
		<button
			class="w-full text-left px-2 py-1.5 text-xs text-text-dim hover:text-text hover:bg-bg-hover rounded transition-colors"
			onclick={() => (workspace.historyPanelOpen = !workspace.historyPanelOpen)}
		>
			{workspace.historyPanelOpen ? '[-] History' : '[+] History'}
		</button>
		<button
			class="w-full text-left px-2 py-1.5 text-xs text-text-dim hover:text-text hover:bg-bg-hover rounded transition-colors"
			onclick={() => (workspace.cmdkOpen = true)}
		>
			[/] Search
		</button>
	</div>
</aside>
