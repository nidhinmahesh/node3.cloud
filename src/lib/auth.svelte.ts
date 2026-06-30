import { api, type User, type TelegramUser } from './api';

class AuthStore {
	user = $state<User | null>(null);
	loading = $state(true);
	initialized = $state(false);

	async init() {
		if (this.initialized) return;
		this.initialized = true;
		if (typeof localStorage === 'undefined') {
			this.loading = false;
			return;
		}
		const token = localStorage.getItem('n3_token');
		if (!token) {
			this.loading = false;
			return;
		}
		try {
			this.user = await api.auth.me();
		} catch {
			localStorage.removeItem('n3_token');
			this.user = null;
		} finally {
			this.loading = false;
		}
	}

	async loginWithTelegram(data: TelegramUser) {
		const res = await api.auth.telegram(data);
		localStorage.setItem('n3_token', res.token);
		this.user = await api.auth.me();
		return this.user;
	}

	async logout() {
		try {
			await api.auth.logout();
		} catch {
			// ignore — clear local state regardless
		}
		localStorage.removeItem('n3_token');
		this.user = null;
		this.initialized = false;
		this.loading = true;
	}

	get isAuthenticated() {
		return this.user !== null;
	}
}

export const auth = new AuthStore();
