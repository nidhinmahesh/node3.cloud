import { redirect } from '@sveltejs/kit';
import { auth } from '$lib/auth.svelte';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ url }) => {
	await auth.init();
	if (!auth.isAuthenticated) {
		redirect(302, '/');
	}
	// Unenrolled users must complete wallet setup before accessing any app route.
	if (!auth.user?.did && url.pathname !== '/setup') {
		redirect(302, '/setup');
	}
};
