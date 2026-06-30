// Wallet: non-custodial BIP-39/BIP-44 key management for node3.cloud.
//
// Key path: m/44'/9999'/0'/0/0 (matching rubixgoplatform LiteDIDMode)
// Public key format sent to node: hex-encoded 65-byte uncompressed secp256k1 point.
// Signature format: ASN.1 DER-encoded ECDSA secp256k1, base64-transmitted.
// Storage: mnemonic encrypted with AES-GCM (PIN-derived via PBKDF2) in IndexedDB.

import { generateMnemonic as bip39Generate, validateMnemonic } from '@scure/bip39';
import { wordlist } from '@scure/bip39/wordlists/english';
import * as secp256k1 from '@noble/secp256k1';
import { HDNodeWallet } from 'ethers';
import Dexie, { type Table } from 'dexie';

// ── Key derivation ────────────────────────────────────────────────────────────

export function generateMnemonic(): string {
	return bip39Generate(wordlist, 128); // 12 words
}

export function validateMnemonicWords(mnemonic: string): boolean {
	return validateMnemonic(mnemonic, wordlist);
}

export interface DerivedKey {
	privateKey: Uint8Array; // 32 bytes — NEVER leaves the browser
	publicKey: Uint8Array;  // 65 bytes uncompressed (0x04 prefix + 32 x + 32 y)
}

export function deriveKey(mnemonic: string): DerivedKey {
	// Use ethers HDNodeWallet for BIP-44 derivation, then extract raw key bytes.
	const wallet = HDNodeWallet.fromPhrase(mnemonic).derivePath("m/44'/9999'/0'/0/0");
	const privateKeyHex = wallet.privateKey.slice(2); // strip 0x
	const privateKey = hexToBytes(privateKeyHex);
	// Derive the uncompressed public key (65 bytes) from the private key.
	const publicKey = secp256k1.getPublicKey(privateKey, false); // false = uncompressed
	return { privateKey, publicKey };
}

// publicKeyToHex returns the hex string that rubixgoplatform's CreateDIDFromPubKey
// expects: 130 hex chars (65 bytes uncompressed secp256k1 point).
export function publicKeyToHex(publicKey: Uint8Array): string {
	return bytesToHex(publicKey);
}

// signHash signs a base64-encoded hash with the private key.
// Returns base64-encoded ASN.1 DER ECDSA signature — the format Go's
// ecdsa.VerifyASN1 expects (used by rubixgoplatform's BIPVerify).
export function signHash(privateKey: Uint8Array, hashBase64: string): string {
	const hashBytes = base64ToBytes(hashBase64);
	const sig = secp256k1.sign(hashBytes, privateKey);
	// toDERRawBytes exists at runtime on both Signature and RecoveredSignature but
	// the TypeScript types for RecoveredSignature don't declare it in this version.
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	return bytesToBase64((sig as any).toDERRawBytes());
}

// ── Encrypted storage ─────────────────────────────────────────────────────────

export interface EncryptedBlob {
	iv: string;   // base64
	salt: string; // base64
	data: string; // base64 ciphertext
}

export async function encryptMnemonic(mnemonic: string, pin: string): Promise<EncryptedBlob> {
	const salt = randomBytes(16);
	const iv = randomBytes(12);
	const key = await deriveAESKey(pin, salt);
	// TextEncoder.encode() returns Uint8Array<ArrayBufferLike> in strict TS mode;
	// cast to silence the BufferSource incompatibility (safe at runtime).
	const encoded = new TextEncoder().encode(mnemonic) as unknown as BufferSource;
	const ciphertext = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, encoded);
	return {
		iv: bytesToBase64(iv),
		salt: bytesToBase64(salt),
		data: bytesToBase64(new Uint8Array(ciphertext))
	};
}

export async function decryptMnemonic(blob: EncryptedBlob, pin: string): Promise<string> {
	// base64ToBytes returns Uint8Array<ArrayBufferLike>; copy into a fresh Uint8Array
	// with a concrete ArrayBuffer so SubtleCrypto's BufferSource check passes.
	const salt = new Uint8Array(base64ToBytes(blob.salt));
	const iv   = new Uint8Array(base64ToBytes(blob.iv));
	const data = new Uint8Array(base64ToBytes(blob.data));
	const key = await deriveAESKey(pin, salt);
	const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, data);
	return new TextDecoder().decode(plain);
}

async function deriveAESKey(pin: string, salt: Uint8Array<ArrayBuffer>): Promise<CryptoKey> {
	const pinBytes = new TextEncoder().encode(pin) as unknown as BufferSource;
	const baseKey = await crypto.subtle.importKey('raw', pinBytes, 'PBKDF2', false, ['deriveKey']);
	return crypto.subtle.deriveKey(
		{ name: 'PBKDF2', salt: salt.buffer, iterations: 200_000, hash: 'SHA-256' },
		baseKey,
		{ name: 'AES-GCM', length: 256 },
		false,
		['encrypt', 'decrypt']
	);
}

// Explicit Uint8Array<ArrayBuffer> return type satisfies SubtleCrypto's BufferSource.
function randomBytes(n: number): Uint8Array<ArrayBuffer> {
	const arr = new Uint8Array(n);
	crypto.getRandomValues(arr);
	return arr;
}

// ── IndexedDB via Dexie ───────────────────────────────────────────────────────

interface WalletRecord {
	accountId: number;
	blob: EncryptedBlob;
}

class WalletDB extends Dexie {
	wallets!: Table<WalletRecord, number>;
	constructor() {
		super('n3_wallet');
		this.version(1).stores({ wallets: 'accountId' });
	}
}

const walletDB = new WalletDB();

export async function storeMnemonic(accountId: number, blob: EncryptedBlob): Promise<void> {
	await walletDB.wallets.put({ accountId, blob });
}

export async function loadMnemonic(accountId: number): Promise<EncryptedBlob | null> {
	const record = await walletDB.wallets.get(accountId);
	return record?.blob ?? null;
}

export async function hasMnemonic(accountId: number): Promise<boolean> {
	const record = await walletDB.wallets.get(accountId);
	return record !== undefined;
}

// ── Byte helpers ──────────────────────────────────────────────────────────────

function hexToBytes(hex: string): Uint8Array {
	const arr = new Uint8Array(hex.length / 2);
	for (let i = 0; i < hex.length; i += 2) {
		arr[i / 2] = parseInt(hex.slice(i, i + 2), 16);
	}
	return arr;
}

function bytesToHex(bytes: Uint8Array): string {
	return Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('');
}

function bytesToBase64(bytes: Uint8Array): string {
	return btoa(String.fromCharCode(...bytes));
}

function base64ToBytes(b64: string): Uint8Array {
	return Uint8Array.from(atob(b64), c => c.charCodeAt(0));
}
