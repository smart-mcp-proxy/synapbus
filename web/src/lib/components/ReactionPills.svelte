<script lang="ts">
	import { reactions as reactionsApi } from '$lib/api/client';

	type ReactionEntry = {
		agent_name: string;
		reaction: string;
		metadata?: Record<string, any>;
	};

	let { reactions = [], messageId }: { reactions: ReactionEntry[]; messageId: number } = $props();

	type GroupedReaction = {
		type: string;
		count: number;
		agents: string[];
		metadata?: Record<string, any>;
	};

	let grouped = $derived((() => {
		const map = new Map<string, GroupedReaction>();
		for (const r of reactions) {
			const existing = map.get(r.reaction);
			if (existing) {
				existing.count++;
				existing.agents.push(r.agent_name);
				if (r.reaction === 'published' && r.metadata?.url) {
					existing.metadata = r.metadata;
				}
			} else {
				map.set(r.reaction, {
					type: r.reaction,
					count: 1,
					agents: [r.agent_name],
					metadata: r.metadata
				});
			}
		}
		return Array.from(map.values());
	})());

	const reactionEmoji: Record<string, string> = {
		approve: '\u2705',
		reject: '\u274C',
		in_progress: '\u23F3',
		done: '\u2714\uFE0F',
		published: '\uD83D\uDE80'
	};

	const reactionColors: Record<string, string> = {
		approve: 'bg-accent-green/15 text-accent-green border-accent-green/30 hover:bg-accent-green/25',
		reject: 'bg-accent-red/15 text-accent-red border-accent-red/30 hover:bg-accent-red/25',
		in_progress: 'bg-accent-blue/15 text-accent-blue border-accent-blue/30 hover:bg-accent-blue/25',
		done: 'bg-bg-tertiary text-text-secondary border-border hover:bg-bg-tertiary/80',
		published: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/30 hover:bg-cyan-500/25'
	};

	let toggling = $state(false);

	async function handleToggle(reactionType: string) {
		if (toggling) return;
		toggling = true;
		try {
			const result = await reactionsApi.toggle(messageId, reactionType);
			// Update local reactions from the server response
			if (result.reactions) {
				reactions = result.reactions;
			}
		} catch {
			// silently fail
		} finally {
			toggling = false;
		}
	}
</script>

{#if grouped.length > 0}
	<div class="flex flex-wrap gap-1 mt-1">
		{#each grouped as group}
			<button
				class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full text-[11px] border transition-colors cursor-pointer {reactionColors[group.type] ?? 'bg-bg-tertiary text-text-secondary border-border'}"
				title={group.agents.join(', ')}
				onclick={(e) => { e.stopPropagation(); handleToggle(group.type); }}
				disabled={toggling}
			>
				<span>{reactionEmoji[group.type] ?? group.type}</span>
				<span class="font-medium">{group.count}</span>
				{#if group.type === 'published' && group.metadata?.url}
					<a
						href={group.metadata.url}
						target="_blank"
						rel="noopener noreferrer"
						class="ml-0.5 hover:underline"
						onclick={(e) => e.stopPropagation()}
						title="View published URL"
					>
						<svg class="w-3 h-3 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
						</svg>
					</a>
				{/if}
			</button>
		{/each}
	</div>
{/if}
