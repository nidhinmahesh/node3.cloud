<script lang="ts">
	let { open = $bindable(false), title = '', children }: {
		open: boolean;
		title?: string;
		children: import('svelte').Snippet;
	} = $props();

	function close() { open = false; }
	function onkeydown(e: KeyboardEvent) { if (e.key === 'Escape') close(); }
</script>

<svelte:window {onkeydown} />

{#if open}
	<!-- backdrop -->
	<div
		class="fixed inset-0 z-40 bg-black/60"
		onclick={close}
		role="presentation"
	></div>

	<!-- panel -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		role="dialog"
		aria-modal="true"
		aria-label={title}
	>
		<div class="w-full max-w-md bg-[--color-bg-surface] border border-[--color-border] rounded-lg shadow-2xl">
			{#if title}
				<div class="flex items-center justify-between px-5 py-4 border-b border-[--color-border]">
					<h2 class="text-sm font-medium text-[--color-text]">{title}</h2>
					<button
						onclick={close}
						class="text-[--color-text-muted] hover:text-[--color-text] transition-colors text-lg leading-none"
						aria-label="close"
					>×</button>
				</div>
			{/if}
			<div class="px-5 py-4">
				{@render children()}
			</div>
		</div>
	</div>
{/if}
