import { writable } from 'svelte/store';

export const activeThread = writable<{ messageId: number; conversationId: number; fromAgent?: string } | null>(null);

export function openThread(messageId: number, conversationId: number, fromAgent?: string) {
	activeThread.set({ messageId, conversationId, fromAgent });
}

export function closeThread() {
	activeThread.set(null);
}
