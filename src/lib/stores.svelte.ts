import { getRecentHistory, type HistoryEntry } from './db';

export type ToolId =
	| 'flow-mapper'
	| 'unit-converter'
	| 'hex-converter'
	| 'base64-codec'
	| 'epoch-converter'
	| 'checksum-tool'
	| 'keccak256'
	| 'function-selector'
	| 'abi-encoder';

export interface ToolDef {
	id: ToolId;
	name: string;
	category: string;
	description: string;
}

export const tools: ToolDef[] = [
	{ id: 'flow-mapper', name: 'Tx Flow', category: 'Visualize', description: 'Map tx flows' },
	{ id: 'unit-converter', name: 'Unit Converter', category: 'Convert', description: 'Wei / Gwei / ETH' },
	{ id: 'hex-converter', name: 'Hex Converter', category: 'Convert', description: 'Hex / Dec / Bin' },
	{ id: 'base64-codec', name: 'Base64 Codec', category: 'Convert', description: 'Base64 / UTF-8 / Hex' },
	{ id: 'epoch-converter', name: 'Epoch Converter', category: 'Convert', description: 'Timestamp / Date' },
	{ id: 'checksum-tool', name: 'Checksum Address', category: 'Convert', description: 'EIP-55 Checksum' },
	{ id: 'keccak256', name: 'Keccak256', category: 'Generate', description: 'Hash any input' },
	{ id: 'function-selector', name: 'Function Selector', category: 'Generate', description: '4-byte selector' },
	{ id: 'abi-encoder', name: 'ABI Encoder', category: 'Decode', description: 'Encode / Decode ABI' }
];

export const categories = ['Visualize', 'Convert', 'Generate', 'Decode'] as const;

class WorkspaceStore {
	activeTool = $state<ToolId>('flow-mapper');
	sidebarOpen = $state(true);
	historyPanelOpen = $state(false);
	cmdkOpen = $state(false);
	history = $state<HistoryEntry[]>([]);
	sendToInput = $state<{ tool: ToolId; value: string } | null>(null);

	async refreshHistory() {
		this.history = await getRecentHistory(100);
	}

	setTool(id: ToolId) {
		this.activeTool = id;
	}

	sendTo(tool: ToolId, value: string) {
		this.sendToInput = { tool, value };
		this.activeTool = tool;
	}
}

export const workspace = new WorkspaceStore();
