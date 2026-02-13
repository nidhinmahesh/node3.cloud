export interface FlowNode {
	id: string;
	address: string;
	label: string;
	x: number;
	y: number;
}

export interface FlowEdge {
	id: string;
	from: string;
	to: string;
	label: string;
}

export interface FlowGraph {
	id?: number;
	name: string;
	nodes: FlowNode[];
	edges: FlowEdge[];
	viewport: { panX: number; panY: number; zoom: number };
	createdAt: string;
	updatedAt: string;
}
