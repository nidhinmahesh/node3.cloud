<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		children
	}: {
		title: string;
		children: Snippet;
	} = $props();
</script>

<div class="w-full max-w-2xl mx-auto">
	<div class="border border-border rounded-lg overflow-hidden">
		<div class="px-4 py-3 border-b border-border">
			<h2 class="text-sm font-semibold text-text">{title}</h2>
		</div>
		<div class="p-4 space-y-4">
			{@render children()}
		</div>
	</div>
</div>
