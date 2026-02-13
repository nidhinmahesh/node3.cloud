import { db } from '$lib/db';
import type { FlowGraph } from './types';

export async function saveGraph(graph: FlowGraph): Promise<number> {
	graph.updatedAt = new Date().toISOString();
	if (graph.id) {
		await db.flow_graphs.put(graph);
		return graph.id;
	}
	graph.createdAt = new Date().toISOString();
	return db.flow_graphs.add(graph) as Promise<number>;
}

export async function loadGraph(id: number): Promise<FlowGraph | undefined> {
	return db.flow_graphs.get(id);
}

export async function listGraphs(): Promise<Pick<FlowGraph, 'id' | 'name' | 'updatedAt'>[]> {
	return db.flow_graphs.orderBy('updatedAt').reverse().toArray().then((gs) =>
		gs.map((g) => ({ id: g.id, name: g.name, updatedAt: g.updatedAt }))
	);
}

export async function deleteGraph(id: number): Promise<void> {
	await db.flow_graphs.delete(id);
}
