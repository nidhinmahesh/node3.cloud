<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/auth.svelte';
	import { api } from '$lib/api';
	import {
		generateMnemonic,
		deriveKey,
		publicKeyToHex,
		encryptMnemonic,
		storeMnemonic
	} from '$lib/wallet';

	type Step = 'generate' | 'confirm' | 'protect' | 'creating' | 'done';

	let step = $state<Step>('generate');
	let mnemonic = $state('');
	let words = $state<string[]>([]);

	// confirm step: ask user to type back 3 random word positions
	let confirmPositions = $state<number[]>([]);
	let confirmAnswers = $state<string[]>(['', '', '']);
	let confirmError = $state('');

	// protect step
	let pin = $state('');
	let pinConfirm = $state('');
	let pinError = $state('');
	let skipPin = $state(false);

	// creating step
	let createError = $state('');

	function start() {
		mnemonic = generateMnemonic();
		words = mnemonic.split(' ');
		// Pick 3 random word positions (1-indexed for display)
		const positions = new Set<number>();
		while (positions.size < 3) {
			positions.add(Math.floor(Math.random() * 12));
		}
		confirmPositions = [...positions].sort((a, b) => a - b);
		confirmAnswers = ['', '', ''];
		step = 'generate';
	}

	function goToConfirm() {
		confirmError = '';
		step = 'confirm';
	}

	function checkConfirm() {
		confirmError = '';
		for (let i = 0; i < confirmPositions.length; i++) {
			if (confirmAnswers[i].trim().toLowerCase() !== words[confirmPositions[i]]) {
				confirmError = `Word ${confirmPositions[i] + 1} is incorrect. Check your backup.`;
				return;
			}
		}
		step = 'protect';
	}

	function goToCreating() {
		pinError = '';
		if (!skipPin) {
			if (pin.length < 6) { pinError = 'PIN must be at least 6 characters.'; return; }
			if (pin !== pinConfirm) { pinError = 'PINs do not match.'; return; }
		}
		step = 'creating';
		createDID();
	}

	async function createDID() {
		createError = '';
		const { privateKey, publicKey } = deriveKey(mnemonic);
		try {
			const pubHex = publicKeyToHex(publicKey);

			// Only store the encrypted mnemonic if the user chose to set a PIN.
			// Users who manage keys externally skip this — they can still use all
			// API features; only browser-based contract signing requires stored keys.
			if (auth.user && !skipPin) {
				const blob = await encryptMnemonic(mnemonic, pin);
				await storeMnemonic(auth.user.id, blob);
			}

			// Register the DID on the node using the public key (non-custodial).
			const result = await api.dids.create(pubHex);

			// Update local auth state so the layout guard doesn't redirect again.
			if (auth.user) {
				auth.user = { ...auth.user, did: result.did };
			}

			step = 'done';
			setTimeout(() => goto('/dashboard'), 1500);
		} catch (e: unknown) {
			createError = e instanceof Error ? e.message : 'Failed to create DID.';
			step = 'protect'; // allow retry
		} finally {
			privateKey.fill(0);
		}
	}

	// Kick off immediately on mount.
	start();
</script>

<div class="min-h-screen flex items-center justify-center p-4 bg-[--color-bg]">
	<div class="w-full max-w-md">

	{#if step === 'generate'}
		<h1 class="text-sm font-medium text-[--color-text] mb-2">Set up your wallet</h1>
		<p class="text-xs text-[--color-text-muted] mb-6 leading-relaxed">
			Your secret recovery phrase is the only way to recover your blockchain identity.
			Write it down and keep it somewhere safe — we never store it.
		</p>
		<div class="grid grid-cols-3 gap-2 mb-6">
			{#each words as word, i}
				<div class="border border-[--color-border] rounded px-2 py-1.5 text-xs flex gap-1.5">
					<span class="text-[--color-text-muted] select-none">{i + 1}.</span>
					<span class="font-mono text-[--color-text]">{word}</span>
				</div>
			{/each}
		</div>
		<p class="text-[10px] text-[--color-yellow] mb-6">
			⚠ Never share this phrase. Anyone with it controls your account.
		</p>
		<button
			onclick={goToConfirm}
			class="w-full text-xs py-2 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors"
		>
			I've written it down →
		</button>

	{:else if step === 'confirm'}
		<h1 class="text-sm font-medium text-[--color-text] mb-2">Confirm your phrase</h1>
		<p class="text-xs text-[--color-text-muted] mb-6">
			Type the words at the positions below to prove you saved your phrase.
		</p>
		<div class="space-y-3 mb-4">
			{#each confirmPositions as pos, i}
				<div>
					<label class="block text-[10px] text-[--color-text-muted] mb-1 uppercase tracking-widest">
						Word #{pos + 1}
					</label>
					<input
						type="text"
						bind:value={confirmAnswers[i]}
						autocomplete="off"
						spellcheck="false"
						class="w-full font-mono text-xs border border-[--color-border] rounded px-2 py-1.5 bg-[--color-bg-surface] text-[--color-text] focus:outline-none focus:border-[--color-accent]"
					/>
				</div>
			{/each}
		</div>
		{#if confirmError}
			<p class="text-xs text-[--color-red] mb-3">{confirmError}</p>
		{/if}
		<div class="flex gap-2">
			<button
				onclick={() => { step = 'generate'; }}
				class="text-xs px-3 py-2 border border-[--color-border] rounded text-[--color-text-muted] hover:text-[--color-text] transition-colors"
			>
				← Back
			</button>
			<button
				onclick={checkConfirm}
				class="flex-1 text-xs py-2 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors"
			>
				Verify →
			</button>
		</div>

	{:else if step === 'protect'}
		<h1 class="text-sm font-medium text-[--color-text] mb-2">Protect your wallet</h1>
		<p class="text-xs text-[--color-text-muted] mb-6 leading-relaxed">
			Set a PIN to encrypt your recovery phrase locally in this browser.
			You'll need it to sign transactions from this browser.
		</p>
		{#if !skipPin}
			<div class="space-y-3 mb-4">
				<div>
					<label class="block text-[10px] text-[--color-text-muted] mb-1 uppercase tracking-widest">
						PIN (min 6 characters)
					</label>
					<input
						type="password"
						bind:value={pin}
						autocomplete="new-password"
						class="w-full text-xs border border-[--color-border] rounded px-2 py-1.5 bg-[--color-bg-surface] text-[--color-text] focus:outline-none focus:border-[--color-accent]"
					/>
				</div>
				<div>
					<label class="block text-[10px] text-[--color-text-muted] mb-1 uppercase tracking-widest">
						Confirm PIN
					</label>
					<input
						type="password"
						bind:value={pinConfirm}
						autocomplete="new-password"
						class="w-full text-xs border border-[--color-border] rounded px-2 py-1.5 bg-[--color-bg-surface] text-[--color-text] focus:outline-none focus:border-[--color-accent]"
					/>
				</div>
			</div>
			{#if pinError}
				<p class="text-xs text-[--color-red] mb-3">{pinError}</p>
			{/if}
		{/if}
		<label class="flex items-center gap-2 text-xs text-[--color-text-muted] mb-6 cursor-pointer">
			<input type="checkbox" bind:checked={skipPin} class="w-3 h-3" />
			Skip — I'll sign transactions with my own wallet
		</label>
		{#if createError}
			<p class="text-xs text-[--color-red] mb-3">{createError}</p>
		{/if}
		<button
			onclick={goToCreating}
			class="w-full text-xs py-2 bg-[--color-accent] text-white rounded hover:bg-[--color-accent-hover] transition-colors"
		>
			Create my DID →
		</button>

	{:else if step === 'creating'}
		<p class="text-xs text-[--color-text-muted]">Creating your DID on the network…</p>

	{:else if step === 'done'}
		<p class="text-xs text-[--color-green]">✓ DID created. Redirecting…</p>
	{/if}

	</div>
</div>
