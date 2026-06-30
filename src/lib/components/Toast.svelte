<script lang="ts">
	export type ToastKind = 'success' | 'error' | 'info';

	let { message = '', kind = 'info' as ToastKind, visible = $bindable(false) }: {
		message: string;
		kind?: ToastKind;
		visible: boolean;
	} = $props();

	const colors: Record<ToastKind, string> = {
		success: 'border-[--color-green] text-[--color-green]',
		error: 'border-[--color-red] text-[--color-red]',
		info: 'border-[--color-accent] text-[--color-accent]'
	};
</script>

{#if visible}
	<div
		class="fixed bottom-6 right-6 z-50 px-4 py-3 text-xs border rounded bg-[--color-bg-surface] shadow-lg {colors[kind]}"
	>
		{message}
	</div>
{/if}
