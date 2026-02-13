import Dexie, { type EntityTable } from 'dexie';
import type { FlowGraph } from '$lib/flow-mapper/types';

export interface HistoryEntry {
	id?: number;
	tool: string;
	input: Record<string, unknown>;
	output: Record<string, unknown>;
	timestamp: string;
	starred: boolean;
}

export interface SavedItem {
	id?: number;
	label: string;
	tool: string;
	data: Record<string, unknown>;
	tags: string[];
	created_at: string;
}

export interface UserPreferences {
	id?: number;
	theme: 'dark' | 'light';
	favorite_tools: string[];
	recent_tools: string[];
}

export interface Snippet {
	id?: number;
	label: string;
	content: string;
	type: string;
	created_at: string;
}

class Node3DB extends Dexie {
	workspace_history!: EntityTable<HistoryEntry, 'id'>;
	saved_items!: EntityTable<SavedItem, 'id'>;
	user_preferences!: EntityTable<UserPreferences, 'id'>;
	snippets!: EntityTable<Snippet, 'id'>;
	flow_graphs!: EntityTable<FlowGraph, 'id'>;

	constructor() {
		super('node3cloud');
		this.version(1).stores({
			workspace_history: '++id, tool, timestamp, starred',
			saved_items: '++id, tool, created_at, *tags',
			user_preferences: '++id',
			snippets: '++id, type, created_at'
		});
		this.version(2).stores({
			workspace_history: '++id, tool, timestamp, starred',
			saved_items: '++id, tool, created_at, *tags',
			user_preferences: '++id',
			snippets: '++id, type, created_at',
			flow_graphs: '++id, name, updatedAt'
		});
	}
}

export const db = new Node3DB();

export async function addHistoryEntry(
	tool: string,
	input: Record<string, unknown>,
	output: Record<string, unknown>
): Promise<number> {
	return db.workspace_history.add({
		tool,
		input,
		output,
		timestamp: new Date().toISOString(),
		starred: false
	});
}

export async function toggleStar(id: number): Promise<void> {
	const entry = await db.workspace_history.get(id);
	if (entry) {
		await db.workspace_history.update(id, { starred: !entry.starred });
	}
}

export async function getRecentHistory(limit = 50): Promise<HistoryEntry[]> {
	return db.workspace_history.orderBy('timestamp').reverse().limit(limit).toArray();
}

export async function getStarredItems(): Promise<HistoryEntry[]> {
	return db.workspace_history.where('starred').equals(1).toArray();
}

export async function saveItem(
	label: string,
	tool: string,
	data: Record<string, unknown>,
	tags: string[] = []
): Promise<number> {
	return db.saved_items.add({
		label,
		tool,
		data,
		tags,
		created_at: new Date().toISOString()
	});
}

export async function exportWorkspace(): Promise<string> {
	const data = {
		version: 1,
		exported_at: new Date().toISOString(),
		history: await db.workspace_history.toArray(),
		saved: await db.saved_items.toArray(),
		snippets: await db.snippets.toArray(),
		preferences: await db.user_preferences.get(1)
	};
	return JSON.stringify(data, null, 2);
}

export async function importWorkspace(json: string): Promise<void> {
	const data = JSON.parse(json);
	if (data.history) await db.workspace_history.bulkAdd(data.history);
	if (data.saved) await db.saved_items.bulkAdd(data.saved);
	if (data.snippets) await db.snippets.bulkAdd(data.snippets);
}

export async function clearHistory(): Promise<void> {
	await db.workspace_history.clear();
}

export async function deleteHistoryEntry(id: number): Promise<void> {
	await db.workspace_history.delete(id);
}
