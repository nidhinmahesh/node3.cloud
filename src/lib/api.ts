// Typed API client — all calls go to /api/* on the same origin (Platform Go service)

function getToken(): string | null {
	if (typeof localStorage === 'undefined') return null;
	return localStorage.getItem('n3_token');
}

function authHeaders(): Record<string, string> {
	const t = getToken();
	return t ? { Authorization: `Bearer ${t}` } : {};
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
	const res = await fetch(path, {
		method,
		headers: { 'Content-Type': 'application/json', ...authHeaders() },
		body: body !== undefined ? JSON.stringify(body) : undefined
	});
	if (!res.ok) {
		const msg = await res.text().catch(() => res.statusText);
		throw new Error(msg || `HTTP ${res.status}`);
	}
	return res.json() as Promise<T>;
}

async function upload<T>(path: string, form: FormData): Promise<T> {
	const res = await fetch(path, { method: 'POST', headers: authHeaders(), body: form });
	if (!res.ok) {
		const msg = await res.text().catch(() => res.statusText);
		throw new Error(msg || `HTTP ${res.status}`);
	}
	return res.json() as Promise<T>;
}

// Request limits per tier. null = no cap (Unlimited plan).
export const TIER_REQUEST_LIMIT: Record<string, number | null> = {
	free:      10_000,
	pro:       500_000,
	unlimited: null
};

export const api = {
	auth: {
		telegram: (data: TelegramUser) =>
			request<{ token: string }>('POST', '/api/auth/telegram', data),
		me: () => request<User>('GET', '/api/auth/me'),
		logout: () => request<void>('POST', '/api/auth/logout')
	},
	keys: {
		list: () => request<Key[]>('GET', '/api/keys'),
		create: (label: string) => request<{ id: string; key: string }>('POST', '/api/keys', { label }),
		revoke: (id: string) => request<void>('DELETE', `/api/keys/${id}`)
	},
	usage: {
		get: () => request<Usage>('GET', '/api/usage')
	},
	webhooks: {
		list: () => request<Webhook[]>('GET', '/api/webhooks'),
		create: (input: CreateWebhookInput) =>
			request<Webhook>('POST', '/api/webhooks', input),
		remove: (id: string) => request<void>('DELETE', `/api/webhooks/${id}`),
		deliveries: (id: string) => request<Delivery[]>('GET', `/api/webhooks/${id}/deliveries`)
	},
	contracts: {
		list: () => request<Contract[]>('GET', '/api/contracts'),
		deploy: (form: FormData) => upload<DeployResult>('/api/contracts/deploy', form),
		executions: (id: string) => request<Execution[]>('GET', `/api/contracts/${id}/executions`)
	},
	dids: {
		create: (publicKey: string) => {
			if (!publicKey) throw new Error('publicKey is required for non-custodial DID creation');
			return request<{ did: string }>('POST', '/api/dids/create', { public_key: publicKey });
		}
	},
	tx: {
		initiate: (body: unknown) => request<unknown>('POST', '/api/tx/initiate', body),
		sign: (id: string, signature: string) =>
			request<{ status: boolean; message: string }>('POST', '/api/tx/sign', { id, signature })
	},
	billing: {
		get: () => request<BillingInfo>('GET', '/api/billing'),
		checkout: (plan: 'pro' | 'unlimited') =>
			request<{ url: string }>('POST', '/api/billing/checkout', { plan }),
		cancel: () => request<void>('POST', '/api/billing/cancel')
	}
};

// ── Types ──────────────────────────────────────────────────────────────────────

export interface TelegramUser {
	id: number;
	first_name: string;
	last_name?: string;
	username?: string;
	photo_url?: string;
	auth_date: number;
	hash: string;
}

export interface User {
	id: number;
	telegram_id: number;
	telegram_username: string;
	did: string;
	tier: 'free' | 'pro' | 'unlimited';
	created_at: string;
}

export interface Key {
	id: string;
	label: string;
	created_at: string;
	revoked_at: string | null;
	last_used_at: string | null;
}

export interface Usage {
	request_count: number;
	limit: number;
	month: string;
	reset_at: string;
}

export interface Webhook {
	id: string;
	event_type: string;
	filter_value: string;
	callback_url: string;
	active: boolean;
	created_at: string;
}

export interface CreateWebhookInput {
	event_type: string;
	filter_value: string;
	callback_url: string;
}

export interface Delivery {
	id: string;
	subscription_id: string;
	transaction_id: string;
	attempted_at: string;
	status: 'success' | 'failed' | 'pending';
	response_code: number | null;
}

export interface Contract {
	id: string;
	contract_id: string;
	deployed_at: string | null;
	execution_count: number;
	current_state: Record<string, unknown>;
}

// DeployResult covers both the immediate-success case (custodial DID) and the
// signature-needed case (non-custodial DID) from POST /api/contracts/deploy.
export interface DeployResult extends Partial<Contract> {
	needs_signature?: boolean;
	sign_id?: string;
	hash?: string;      // base64-encoded hash to sign
	contract_id?: string;
}

export interface Execution {
	id: string;
	executed_at: string;
	initiator_did: string;
	input: unknown;
	output: unknown;
	state_before: unknown;
	state_after: unknown;
	success: boolean;
	error: string | null;
}

export interface BillingInfo {
	tier: 'free' | 'pro' | 'unlimited';
	subscription_id: string | null;
	next_billing_date: string | null;
	cancel_at: string | null;
}
