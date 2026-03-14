<script lang="ts">
	import { deadLetters as deadLettersApi } from '$lib/api/client';

	let letterList = $state<any[]>([]);
	let loadingData = $state(true);
	let showAcknowledged = $state(false);
	let totalUnacknowledged = $state(0);

	let _initialized = $state(false);
	$effect(() => {
		if (!_initialized) {
			_initialized = true;
			loadDeadLetters();
		}
	});

	async function loadDeadLetters() {
		loadingData = true;
		try {
			const res = await deadLettersApi.list({ acknowledged: showAcknowledged, limit: 100 });
			letterList = res.dead_letters;
			totalUnacknowledged = res.total;
		} catch {
			// handled
		} finally {
			loadingData = false;
		}
	}

	async function acknowledge(id: number) {
		try {
			await deadLettersApi.acknowledge(id);
			// Remove from list or mark as acknowledged
			letterList = letterList.map((dl) =>
				dl.id === id ? { ...dl, acknowledged: true } : dl
			);
			if (!showAcknowledged) {
				letterList = letterList.filter((dl) => !dl.acknowledged);
			}
			totalUnacknowledged = Math.max(0, totalUnacknowledged - 1);
		} catch {
			// handled
		}
	}

	function toggleShowAcknowledged() {
		showAcknowledged = !showAcknowledged;
		loadDeadLetters();
	}

	function formatTime(ts: string): string {
		return new Date(ts).toLocaleString();
	}

	function truncateBody(body: string, maxLen = 120): string {
		if (body.length <= maxLen) return body;
		return body.slice(0, maxLen) + '...';
	}

	function priorityLabel(p: number): string {
		if (p >= 8) return 'Urgent';
		if (p >= 5) return 'High';
		if (p >= 3) return 'Normal';
		return 'Low';
	}

	function priorityColor(p: number): string {
		if (p >= 8) return 'text-accent-red';
		if (p >= 5) return 'text-accent-orange';
		return 'text-text-secondary';
	}
</script>

<div class="p-5 max-w-5xl">
	<div class="flex items-center justify-between mb-5">
		<div class="flex items-center gap-3">
			<h1 class="text-xl font-bold text-text-primary font-display">Dead Letters</h1>
			{#if totalUnacknowledged > 0}
				<span class="badge bg-accent-red/20 text-accent-red">{totalUnacknowledged} unacknowledged</span>
			{/if}
		</div>
		<button
			class="text-xs px-3 py-1.5 rounded border border-border text-text-secondary hover:text-text-primary hover:border-text-secondary transition-colors"
			onclick={toggleShowAcknowledged}
		>
			{showAcknowledged ? 'Hide acknowledged' : 'Show acknowledged'}
		</button>
	</div>

	<div class="card">
		<div class="px-5 py-3 border-b border-border">
			<h2 class="font-semibold text-sm text-text-primary font-display">
				{showAcknowledged ? 'All Dead Letters' : 'Unacknowledged Dead Letters'}
			</h2>
			<p class="text-xs text-text-secondary mt-0.5">
				Messages that could not be delivered because the recipient agent was deleted.
			</p>
		</div>

		{#if loadingData}
			<div class="p-5 space-y-4">
				{#each Array(3) as _}
					<div class="space-y-2">
						<div class="skeleton h-3 w-1/3"></div>
						<div class="skeleton h-3 w-2/3"></div>
					</div>
				{/each}
			</div>
		{:else if letterList.length === 0}
			<div class="p-8 text-center">
				<svg class="w-12 h-12 mx-auto mb-3 text-text-secondary/40" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
					<path stroke-linecap="round" stroke-linejoin="round" d="M21.75 9v.906a2.25 2.25 0 01-1.183 1.981l-6.478 3.488M2.25 9v.906a2.25 2.25 0 001.183 1.981l6.478 3.488m8.839 2.51l-4.66-2.51m0 0l-1.023-.55a2.25 2.25 0 00-2.134 0l-1.022.55m0 0l-4.661 2.51m16.5 1.615a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V8.844a2.25 2.25 0 011.183-1.98l7.5-4.04a2.25 2.25 0 012.134 0l7.5 4.04a2.25 2.25 0 011.183 1.98V19.5z" />
				</svg>
				<p class="text-text-secondary text-sm">No dead letters.</p>
				<p class="text-text-secondary/60 text-xs mt-1">Messages appear here when an agent is deleted with pending messages.</p>
			</div>
		{:else}
			<div class="divide-y divide-border">
				{#each letterList as dl (dl.id)}
					<div class="px-5 py-3 {dl.acknowledged ? 'opacity-50' : ''}">
						<div class="flex items-start justify-between gap-3">
							<div class="min-w-0 flex-1">
								<div class="flex items-center gap-2 mb-1 flex-wrap">
									<span class="font-semibold text-sm text-text-primary">{dl.from_agent}</span>
									<svg class="w-3 h-3 text-text-secondary flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
									</svg>
									<span class="font-semibold text-sm text-accent-red/80 line-through">{dl.to_agent}</span>
									<span class="text-[9px] text-accent-red bg-accent-red/10 px-1 rounded">deleted</span>
									{#if dl.priority > 0}
										<span class="text-[10px] {priorityColor(dl.priority)}">{priorityLabel(dl.priority)}</span>
									{/if}
									<span class="text-xs text-text-secondary ml-auto flex-shrink-0">{formatTime(dl.created_at)}</span>
								</div>
								{#if dl.subject}
									<p class="text-xs font-medium text-text-secondary mb-0.5">{dl.subject}</p>
								{/if}
								<p class="text-sm text-text-primary/80">{truncateBody(dl.body)}</p>
							</div>
							{#if !dl.acknowledged}
								<button
									class="btn-primary text-xs flex-shrink-0 px-3 py-1.5"
									onclick={() => acknowledge(dl.id)}
								>
									Acknowledge
								</button>
							{:else}
								<span class="text-[10px] text-text-secondary bg-bg-tertiary px-2 py-1 rounded flex-shrink-0">Acknowledged</span>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>
