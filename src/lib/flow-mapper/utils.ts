const CHARS = 'abcdefghijklmnopqrstuvwxyz0123456789';

function randomChars(n: number): string {
	let s = '';
	for (let i = 0; i < n; i++) s += CHARS[Math.floor(Math.random() * CHARS.length)];
	return s;
}

export function nodeId(): string {
	return 'nd_' + randomChars(8);
}

export function edgeId(): string {
	return 'ed_' + randomChars(8);
}

export const NODE_W = 180;
export const NODE_H = 56;
export const PORT_R = 5;
export const MIN_ZOOM = 0.15;
export const MAX_ZOOM = 3;

export function portOut(x: number, y: number): { x: number; y: number } {
	return { x: x + NODE_W, y: y + NODE_H / 2 };
}

export function portIn(x: number, y: number): { x: number; y: number } {
	return { x, y: y + NODE_H / 2 };
}

export function bezierPath(
	x1: number,
	y1: number,
	x2: number,
	y2: number
): string {
	const dx = Math.abs(x2 - x1) * 0.5;
	return `M${x1},${y1} C${x1 + dx},${y1} ${x2 - dx},${y2} ${x2},${y2}`;
}

export function screenToWorld(
	sx: number,
	sy: number,
	panX: number,
	panY: number,
	zoom: number
): { x: number; y: number } {
	return { x: (sx - panX) / zoom, y: (sy - panY) / zoom };
}
