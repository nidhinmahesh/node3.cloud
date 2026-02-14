<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import HistoryPanel from '$lib/components/HistoryPanel.svelte';
	import CommandPalette from '$lib/components/CommandPalette.svelte';
	import UnitConverter from '$lib/tools/UnitConverter.svelte';
	import HexConverter from '$lib/tools/HexConverter.svelte';
	import Base64Codec from '$lib/tools/Base64Codec.svelte';
	import EpochConverter from '$lib/tools/EpochConverter.svelte';
	import ChecksumTool from '$lib/tools/ChecksumTool.svelte';
	import Keccak256 from '$lib/tools/Keccak256.svelte';
	import FunctionSelector from '$lib/tools/FunctionSelector.svelte';
	import AbiEncoder from '$lib/tools/AbiEncoder.svelte';
	import FlowMapperCanvas from '$lib/flow-mapper/FlowMapperCanvas.svelte';
	import { workspace } from '$lib/stores.svelte';
	import { exportWorkspace, importWorkspace } from '$lib/db';

	onMount(() => {
		workspace.refreshHistory();

		function handleKeydown(e: KeyboardEvent) {
			if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
				e.preventDefault();
				workspace.cmdkOpen = !workspace.cmdkOpen;
			}
		}
		window.addEventListener('keydown', handleKeydown);
		return () => window.removeEventListener('keydown', handleKeydown);
	});

	async function handleExport() {
		const json = await exportWorkspace();
		const blob = new Blob([json], { type: 'application/json' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `node3-workspace-${Date.now()}.json`;
		a.click();
		URL.revokeObjectURL(url);
	}

	async function handleImport() {
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = '.json';
		input.onchange = async () => {
			const file = input.files?.[0];
			if (!file) return;
			const text = await file.text();
			await importWorkspace(text);
			await workspace.refreshHistory();
		};
		input.click();
	}
</script>

<div class="flex h-screen bg-bg overflow-hidden">
	<Sidebar />

	{#if workspace.activeTool === 'flow-mapper'}
		<main class="flex-1 overflow-hidden">
			<!-- mobile menu button for flow mapper -->
			<button
				class="fixed top-2 right-2 z-30 md:hidden text-[10px] px-2 py-1 rounded border border-border bg-bg-surface text-text-muted hover:text-text"
				onclick={() => (workspace.sidebarOpen = true)}
			>menu</button>
			<FlowMapperCanvas />
		</main>
	{:else}
		<main class="flex-1 flex flex-col overflow-y-auto p-3 md:p-6">
			<div class="flex items-center gap-2 mb-4 md:mb-6">
				<button
					class="md:hidden text-[10px] px-2 py-1 rounded border border-border text-text-muted hover:text-text hover:border-border-bright transition-colors"
					onclick={() => (workspace.sidebarOpen = true)}
				>menu</button>
				<div class="flex items-center gap-2 ml-auto">
					<button
						class="text-[10px] px-2 py-1 rounded border border-border text-text-muted hover:text-text hover:border-border-bright transition-colors"
						onclick={handleExport}
					>export</button>
					<button
						class="text-[10px] px-2 py-1 rounded border border-border text-text-muted hover:text-text hover:border-border-bright transition-colors"
						onclick={handleImport}
					>import</button>
					<span class="text-[10px] text-text-muted hidden sm:inline">Cmd+K search</span>
				</div>
			</div>

			{#if workspace.activeTool === 'unit-converter'}
				<UnitConverter />
			{:else if workspace.activeTool === 'hex-converter'}
				<HexConverter />
			{:else if workspace.activeTool === 'base64-codec'}
				<Base64Codec />
			{:else if workspace.activeTool === 'epoch-converter'}
				<EpochConverter />
			{:else if workspace.activeTool === 'checksum-tool'}
				<ChecksumTool />
			{:else if workspace.activeTool === 'keccak256'}
				<Keccak256 />
			{:else if workspace.activeTool === 'function-selector'}
				<FunctionSelector />
			{:else if workspace.activeTool === 'abi-encoder'}
				<AbiEncoder />
			{/if}

			<footer class="mt-auto pt-12 pb-4 text-center">
				<p class="text-[11px] text-text-dim leading-relaxed">Your personal web3 dev workbench — tools, history, and AI help, all saved in your browser.<br/>You don't need cloud for everything just like the name.</p>
				<p class="text-[10px] text-text-muted mt-2">sponsored by <a href="https://getfexr.com" target="_blank" rel="noopener noreferrer" class="text-accent hover:underline">Fexr</a></p>
				<a href="https://github.com/nidhinmahesh/node3.cloud" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 text-[10px] text-text-muted hover:text-accent mt-1" title="Contribute on GitHub">
					<svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
					open source — contribute
				</a>
			</footer>
		</main>
	{/if}

	{#if workspace.historyPanelOpen}
		<HistoryPanel />
	{/if}
</div>

<CommandPalette />
