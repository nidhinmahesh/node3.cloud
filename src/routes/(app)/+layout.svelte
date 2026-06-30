<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	let { children } = $props();
	let sidebarOpen = $state(false);
</script>

<div class="flex h-screen bg-[--color-bg] text-[--color-text] overflow-hidden">
	<Sidebar bind:open={sidebarOpen} />
	<div class="flex-1 flex flex-col overflow-hidden min-w-0">
		<!-- Mobile top bar with hamburger -->
		<div class="md:hidden flex items-center px-4 py-3 border-b border-[--color-border] shrink-0">
			<button
				onclick={() => { sidebarOpen = true; }}
				class="text-[--color-text-muted] hover:text-[--color-text] transition-colors text-base leading-none mr-3"
				aria-label="open menu"
			>☰</button>
			<span class="text-xs text-[--color-text-dim]">node3.cloud</span>
		</div>
		<main class="flex-1 overflow-y-auto">
			{@render children()}
		</main>
	</div>
</div>
