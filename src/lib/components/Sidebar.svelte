<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const links = [
		{ href: '/dashboard', label: 'dashboard' },
		{ href: '/keys', label: 'api keys' },
		{ href: '/webhooks', label: 'webhooks' },
		{ href: '/contracts', label: 'contracts' },
		{ href: '/billing', label: 'billing' }
	];

	function isActive(href: string) {
		return $page.url.pathname === href;
	}

	async function handleLogout() {
		await auth.logout();
		open = false;
		goto('/');
	}

	function close() {
		open = false;
	}
</script>

<!-- Mobile overlay backdrop -->
{#if open}
	<div
		class="fixed inset-0 bg-black/50 z-40 md:hidden"
		onclick={close}
		role="presentation"
	></div>
{/if}

<aside class="w-48 shrink-0 flex flex-col border-r border-[--color-border] bg-[--color-bg]
	fixed z-50 top-0 left-0 h-full transition-transform duration-200
	md:static md:translate-x-0 md:h-auto
	{open ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}">
	<!-- logo -->
	<div class="px-4 py-5 border-b border-[--color-border] flex items-center justify-between">
		<a href="/" class="text-sm font-medium text-[--color-text] hover:text-[--color-accent] transition-colors">
			node3.cloud
		</a>
		<button
			onclick={close}
			class="md:hidden text-[--color-text-muted] hover:text-[--color-text] transition-colors text-lg leading-none"
			aria-label="close menu"
		>×</button>
	</div>

	<!-- nav -->
	<nav class="flex-1 py-3 overflow-y-auto">
		{#each links as link}
			<a
				href={link.href}
				onclick={close}
				class="flex items-center px-4 py-2 text-xs transition-colors
					{isActive(link.href)
						? 'text-[--color-text] bg-[--color-bg-active]'
						: 'text-[--color-text-dim] hover:text-[--color-text] hover:bg-[--color-bg-hover]'}"
			>
				{link.label}
			</a>
		{/each}
	</nav>

	<!-- user footer -->
	{#if auth.user}
		<div class="px-4 py-4 border-t border-[--color-border]">
			<p class="text-[10px] text-[--color-text-muted] mb-1 truncate">
				@{auth.user.telegram_username || auth.user.telegram_id}
			</p>
			<p
				class="text-[10px] text-[--color-text-muted] mb-3 truncate"
				title={auth.user.did}
			>
				{auth.user.did ? auth.user.did.slice(0, 16) + '…' : '—'}
			</p>
			<button
				onclick={handleLogout}
				class="text-[10px] text-[--color-text-muted] hover:text-[--color-red] transition-colors"
			>
				logout
			</button>
		</div>
	{/if}
</aside>
