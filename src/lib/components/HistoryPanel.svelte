<script lang="ts">
	import { workspace, tools, type ToolId } from '$lib/stores.svelte';
	import { toggleStar, deleteHistoryEntry } from '$lib/db';

	function toolName(id: string): string {
		return tools.find((t) => t.id === id)?.name ?? id;
	}

	function formatTime(iso: string): string {
		return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	function preview(entry: { input: Record<string, unknown>; output: Record<string, unknown> }): string {
		const inp = Object.values(entry.input)[0];
		const out = Object.values(entry.output)[0];
		const inStr = String(inp ?? '').slice(0, 20);
		const outStr = String(out ?? '').slice(0, 20);
		return `${inStr} → ${outStr}`;
	}

	async function handleStar(id: number) {
		await toggleStar(id);
		await workspace.refreshHistory();
	}

	async function handleDelete(id: number) {
		await deleteHistoryEntry(id);
		await workspace.refreshHistory();
	}
</script>

<aside class="w-72 h-full bg-bg-surface border-l border-border flex flex-col shrink-0
	fixed z-50 top-0 right-0 md:static">
	<div class="px-4 py-3 border-b border-border flex items-center justify-between">
		<span class="text-xs font-semibold text-text-dim uppercase tracking-widest">History</span>
		<button
			class="text-xs text-text-muted hover:text-text"
			onclick={() => (workspace.historyPanelOpen = false)}
		>
			×
		</button>
	</div>
	<div class="flex-1 overflow-y-auto">
		{#if workspace.history.length === 0}
			<p class="p-4 text-xs text-text-muted">No history yet. Start using a tool.</p>
		{/if}
		{#each workspace.history as entry (entry.id)}
			<div class="px-3 py-2 border-b border-border hover:bg-bg-hover group">
				<div class="flex items-center justify-between">
					<span class="text-[10px] text-text-muted">{formatTime(entry.timestamp)}</span>
					<div class="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
						<button
							class="text-[10px] {entry.starred ? 'text-yellow' : 'text-text-muted'} hover:text-yellow"
							onclick={() => entry.id && handleStar(entry.id)}
						>*</button>
						<button
							class="text-[10px] text-text-muted hover:text-red"
							onclick={() => entry.id && handleDelete(entry.id)}
						>x</button>
					</div>
				</div>
				<button
					class="text-left w-full"
					onclick={() => workspace.setTool(entry.tool as ToolId)}
				>
					<p class="text-[11px] text-accent">{toolName(entry.tool)}</p>
					<p class="text-[10px] text-text-dim truncate">{preview(entry)}</p>
				</button>
			</div>
		{/each}
	</div>
</aside>
