<script lang="ts">
	import { workspace, tools, type ToolId } from '$lib/stores.svelte';
	import { onMount } from 'svelte';

	let query = $state('');
	let inputEl = $state<HTMLInputElement | null>(null);

	const filtered = $derived(
		tools.filter(
			(t) =>
				t.name.toLowerCase().includes(query.toLowerCase()) ||
				t.description.toLowerCase().includes(query.toLowerCase()) ||
				t.category.toLowerCase().includes(query.toLowerCase())
		)
	);

	function select(id: ToolId) {
		workspace.setTool(id);
		workspace.cmdkOpen = false;
		query = '';
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			workspace.cmdkOpen = false;
			query = '';
		}
		if (e.key === 'Enter' && filtered.length > 0) {
			select(filtered[0].id);
		}
	}

	onMount(() => {
		inputEl?.focus();
	});
</script>

{#if workspace.cmdkOpen}
	<!-- svelte-ignore a11y_interactive_supports_focus -->
	<div
		class="fixed inset-0 bg-black/60 z-50 flex items-start justify-center pt-[20vh]"
		onkeydown={handleKeydown}
		role="dialog"
		aria-label="Search tools"
	>
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div
			class="bg-bg-surface border border-border rounded-lg w-[420px] shadow-2xl overflow-hidden"
			onclick={(e) => e.stopPropagation()}
			role="listbox"
		>
			<input
				bind:this={inputEl}
				bind:value={query}
				placeholder="Search tools..."
				class="w-full border-0 border-b border-border bg-transparent px-4 py-3 text-sm focus:outline-none"
			/>
			<div class="max-h-64 overflow-y-auto">
				{#each filtered as tool}
					<button
						class="w-full text-left px-4 py-2.5 hover:bg-bg-hover flex justify-between items-center"
						onclick={() => select(tool.id)}
						role="option"
						aria-selected={false}
					>
						<span class="text-xs text-text">{tool.name}</span>
						<span class="text-[10px] text-text-muted">{tool.category}</span>
					</button>
				{/each}
				{#if filtered.length === 0}
					<p class="px-4 py-3 text-xs text-text-muted">No matching tools.</p>
				{/if}
			</div>
		</div>
	</div>
{/if}
