<script lang="ts">
	import { page } from '$app/stores';
	import { runs as runsApi } from '$lib/api/client';

	type Run = {
		id: number;
		agent_name: string;
		status: string;
		trigger_event: string;
		trigger_from: string;
		trigger_depth: number;
		trigger_message_id?: number | null;
		k8s_job_name?: string;
		duration_ms?: number | null;
		started_at?: string;
		completed_at?: string;
		created_at: string;
		error_log?: string;
	};

	type HarnessRun = {
		id: number;
		run_id: string;
		backend: string;
		status: string;
		exit_code?: number | null;
		trace_id?: string;
		session_id?: string;
		tokens_in: number;
		tokens_out: number;
		tokens_cached: number;
		cost_usd: number;
		duration_ms?: number | null;
		logs_excerpt?: string;
		prompt?: string;
		response?: string;
		result_json?: string;
		created_at: string;
		finished_at?: string;
	};

	type AgentSnapshot = {
		name: string;
		display_name?: string;
		type: string;
		harness_name?: string;
		local_command?: string;
		harness_config_json?: string;
		trigger_mode: string;
		cooldown_seconds: number;
		daily_trigger_budget: number;
		max_trigger_depth: number;
	};

	type TriggerMessage = {
		id: number;
		from_agent: string;
		to_agent?: string;
		body: string;
		priority: number;
		status: string;
		created_at: string;
	};

	type OutgoingMessage = {
		id: number;
		to_agent?: string;
		body: string;
		status: string;
		created_at: string;
	};

	let loading = $state(true);
	let error = $state('');
	let run = $state<Run | null>(null);
	let harnessRun = $state<HarnessRun | null>(null);
	let agent = $state<AgentSnapshot | null>(null);
	let triggerMessage = $state<TriggerMessage | null>(null);
	let outgoingMessage = $state<OutgoingMessage | null>(null);
	let harnessConfigParsed = $state<any>(null);

	let runId = $derived(parseInt($page.params.id, 10));

	async function load() {
		loading = true;
		error = '';
		try {
			const res: any = await runsApi.get(runId);
			run = res.run;
			harnessRun = res.harness_run ?? null;
			agent = res.agent ?? null;
			triggerMessage = res.trigger_message ?? null;
			outgoingMessage = res.outgoing_message ?? null;
			if (agent?.harness_config_json) {
				try {
					harnessConfigParsed = JSON.parse(agent.harness_config_json);
				} catch {
					harnessConfigParsed = null;
				}
			}
		} catch (err: any) {
			error = err?.message || 'Failed to load run';
		} finally {
			loading = false;
		}
	}

	let _initialized = $state(false);
	$effect(() => {
		if (!_initialized && !Number.isNaN(runId)) {
			_initialized = true;
			load();
		}
	});

	function statusDot(status: string): string {
		switch (status) {
			case 'running': return 'bg-accent-blue animate-pulse';
			case 'succeeded':
			case 'success': return 'bg-accent-green';
			case 'failed': return 'bg-accent-red';
			case 'queued':
			case 'cooldown_skipped':
			case 'budget_exhausted':
			case 'depth_exceeded': return 'bg-accent-yellow';
			default: return 'bg-text-secondary';
		}
	}

	function statusBadgeClass(status: string): string {
		switch (status) {
			case 'running': return 'bg-accent-blue/15 text-accent-blue border-accent-blue/30';
			case 'succeeded':
			case 'success': return 'bg-accent-green/15 text-accent-green border-accent-green/30';
			case 'failed': return 'bg-accent-red/15 text-accent-red border-accent-red/30';
			default: return 'bg-bg-tertiary text-text-secondary border-border';
		}
	}

	function formatDuration(ms?: number | null): string {
		if (ms == null) return '—';
		if (ms < 1000) return `${ms}ms`;
		const s = ms / 1000;
		if (s < 60) return `${s.toFixed(1)}s`;
		const m = s / 60;
		return `${m.toFixed(1)}m`;
	}

	function formatCost(usd: number): string {
		if (usd < 0.0001) return `$${usd.toFixed(6)}`;
		if (usd < 0.01) return `$${usd.toFixed(4)}`;
		return `$${usd.toFixed(3)}`;
	}

	function formatTokens(n: number): string {
		if (n < 1000) return String(n);
		return `${(n / 1000).toFixed(1)}k`;
	}

	async function copy(text: string) {
		try { await navigator.clipboard.writeText(text); } catch {}
	}
</script>

<div class="p-5 max-w-5xl mx-auto">
	<a href="/runs" class="inline-flex items-center gap-1 text-xs text-text-link hover:underline mb-4">
		<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
			<path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
		</svg>
		Back to runs
	</a>

	{#if loading}
		<div class="card p-8 text-center text-text-secondary text-sm">Loading run #{runId}…</div>
	{:else if error}
		<div class="card p-6 border border-accent-red/30">
			<p class="text-sm text-accent-red font-semibold mb-1">Failed to load run</p>
			<p class="text-xs text-text-secondary font-mono">{error}</p>
		</div>
	{:else if run}
		<!-- Header strip -->
		<div class="card mb-5">
			<div class="p-5 border-b border-border flex items-start justify-between flex-wrap gap-3">
				<div class="flex items-center gap-3">
					<span class="w-2.5 h-2.5 rounded-full {statusDot(run.status)}"></span>
					<div>
						<h1 class="text-lg font-bold text-text-primary font-display">
							<a href="/agents/{run.agent_name}" class="hover:text-accent-blue">{run.agent_name}</a>
							<span class="text-text-secondary font-mono text-sm ml-2">#{run.id}</span>
						</h1>
						<p class="text-xs text-text-secondary font-mono mt-0.5">
							triggered by <a href="/agents/{run.trigger_from}" class="text-text-link hover:underline">@{run.trigger_from}</a>
							&middot; {run.trigger_event}
							&middot; depth {run.trigger_depth}
						</p>
					</div>
				</div>
				<div class="flex items-center gap-2">
					<span class="badge border {statusBadgeClass(run.status)}">{run.status}</span>
					{#if harnessRun}
						<span class="badge bg-bg-tertiary text-text-secondary border border-border font-mono text-[10px]">{harnessRun.backend}</span>
					{/if}
				</div>
			</div>

			<!-- Metrics grid -->
			<div class="p-5 grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
				<div>
					<p class="text-[10px] uppercase tracking-wider text-text-secondary">Duration</p>
					<p class="text-text-primary font-mono">{formatDuration(run.duration_ms)}</p>
				</div>
				<div>
					<p class="text-[10px] uppercase tracking-wider text-text-secondary">Tokens in → out</p>
					<p class="text-text-primary font-mono">
						{harnessRun ? `${formatTokens(harnessRun.tokens_in)} → ${formatTokens(harnessRun.tokens_out)}` : '—'}
					</p>
				</div>
				<div>
					<p class="text-[10px] uppercase tracking-wider text-text-secondary">Cost</p>
					<p class="text-text-primary font-mono">
						{harnessRun ? formatCost(harnessRun.cost_usd) : '—'}
					</p>
				</div>
				<div>
					<p class="text-[10px] uppercase tracking-wider text-text-secondary">Exit</p>
					<p class="text-text-primary font-mono">
						{harnessRun?.exit_code != null ? harnessRun.exit_code : '—'}
					</p>
				</div>
				{#if run.started_at}
					<div>
						<p class="text-[10px] uppercase tracking-wider text-text-secondary">Started</p>
						<p class="text-text-primary text-xs">{new Date(run.started_at).toLocaleString()}</p>
					</div>
				{/if}
				{#if run.completed_at}
					<div>
						<p class="text-[10px] uppercase tracking-wider text-text-secondary">Completed</p>
						<p class="text-text-primary text-xs">{new Date(run.completed_at).toLocaleString()}</p>
					</div>
				{/if}
				{#if harnessRun?.trace_id}
					<div class="col-span-2">
						<p class="text-[10px] uppercase tracking-wider text-text-secondary">Trace ID</p>
						<button
							class="text-text-primary font-mono text-[11px] hover:text-accent-blue truncate block w-full text-left"
							title="Copy"
							onclick={() => copy(harnessRun?.trace_id || '')}
						>{harnessRun.trace_id}</button>
					</div>
				{/if}
			</div>
		</div>

		<!-- Triggering message -->
		{#if triggerMessage}
			<div class="card mb-5">
				<div class="px-5 py-3 border-b border-border flex items-center gap-2">
					<svg class="w-4 h-4 text-accent-blue" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
					</svg>
					<h2 class="font-semibold text-sm text-text-primary font-display">Triggering message</h2>
					<span class="ml-auto text-[10px] font-mono text-text-secondary">#{triggerMessage.id}</span>
				</div>
				<div class="p-5">
					<p class="text-xs text-text-secondary mb-1">
						from <span class="text-text-primary font-mono">@{triggerMessage.from_agent}</span>
						{#if triggerMessage.to_agent} → <span class="text-text-primary font-mono">@{triggerMessage.to_agent}</span>{/if}
						&middot; priority {triggerMessage.priority}
					</p>
					<pre class="text-sm text-text-primary bg-bg-primary border border-border rounded-lg p-3 whitespace-pre-wrap font-mono leading-relaxed">{triggerMessage.body}</pre>
				</div>
			</div>
		{/if}

		<!-- What the model saw: system instructions + rendered prompt -->
		<div class="card mb-5">
			<div class="px-5 py-3 border-b border-border flex items-center gap-2">
				<svg class="w-4 h-4 text-accent-purple" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
					<path stroke-linecap="round" stroke-linejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
				</svg>
				<h2 class="font-semibold text-sm text-text-primary font-display">What the model saw</h2>
			</div>
			<div class="p-5 space-y-4">
				{#if harnessConfigParsed?.gemini_md}
					<details class="bg-bg-primary border border-border rounded-lg" open>
						<summary class="px-3 py-2 text-xs text-text-secondary cursor-pointer hover:text-text-primary flex items-center justify-between">
							<span>GEMINI.md (agent's current system instructions)</span>
							<span class="font-mono text-[10px]">{harnessConfigParsed.gemini_md.length} bytes</span>
						</summary>
						<pre class="px-3 pb-3 text-xs font-mono text-text-primary/80 whitespace-pre-wrap leading-relaxed">{harnessConfigParsed.gemini_md}</pre>
					</details>
				{/if}

				{#if harnessConfigParsed?.claude_md}
					<details class="bg-bg-primary border border-border rounded-lg">
						<summary class="px-3 py-2 text-xs text-text-secondary cursor-pointer hover:text-text-primary flex items-center justify-between">
							<span>CLAUDE.md (agent's current system instructions)</span>
							<span class="font-mono text-[10px]">{harnessConfigParsed.claude_md.length} bytes</span>
						</summary>
						<pre class="px-3 pb-3 text-xs font-mono text-text-primary/80 whitespace-pre-wrap leading-relaxed">{harnessConfigParsed.claude_md}</pre>
					</details>
				{/if}

				{#if harnessRun?.prompt}
					<details class="bg-bg-primary border border-border rounded-lg" open>
						<summary class="px-3 py-2 text-xs text-text-secondary cursor-pointer hover:text-text-primary flex items-center justify-between">
							<span>Rendered prompt (captured from subprocess workdir)</span>
							<span class="font-mono text-[10px]">{harnessRun.prompt.length} bytes</span>
						</summary>
						<pre class="px-3 pb-3 text-xs font-mono text-text-primary/80 whitespace-pre-wrap leading-relaxed">{harnessRun.prompt}</pre>
					</details>
				{:else}
					<p class="text-xs text-text-secondary italic">No captured prompt — this run's backend does not record prompts, or the wrapper did not write <code class="font-mono">prompt.txt</code>.</p>
				{/if}
			</div>
		</div>

		<!-- What the model said: raw response -->
		<div class="card mb-5">
			<div class="px-5 py-3 border-b border-border flex items-center gap-2">
				<svg class="w-4 h-4 text-accent-green" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
				</svg>
				<h2 class="font-semibold text-sm text-text-primary font-display">What the model said</h2>
			</div>
			<div class="p-5">
				{#if harnessRun?.response}
					<pre class="text-sm text-text-primary bg-bg-primary border border-border rounded-lg p-3 whitespace-pre-wrap font-mono leading-relaxed max-h-96 overflow-y-auto">{harnessRun.response}</pre>
				{:else if harnessRun?.logs_excerpt}
					<p class="text-xs text-text-secondary mb-2 italic">No captured response field — showing bounded logs excerpt instead.</p>
					<pre class="text-xs text-text-primary/80 bg-bg-primary border border-border rounded-lg p-3 whitespace-pre-wrap font-mono leading-relaxed max-h-96 overflow-y-auto">{harnessRun.logs_excerpt}</pre>
				{:else if run.error_log}
					<p class="text-xs text-accent-red mb-2 font-semibold">Error log</p>
					<pre class="text-xs text-accent-red bg-accent-red/5 border border-accent-red/30 rounded-lg p-3 whitespace-pre-wrap font-mono leading-relaxed">{run.error_log}</pre>
				{:else}
					<p class="text-xs text-text-secondary italic">Nothing captured.</p>
				{/if}
			</div>
		</div>

		<!-- Outgoing message -->
		{#if outgoingMessage}
			<div class="card mb-5">
				<div class="px-5 py-3 border-b border-border flex items-center gap-2">
					<svg class="w-4 h-4 text-accent-yellow" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
					</svg>
					<h2 class="font-semibold text-sm text-text-primary font-display">Outgoing message</h2>
					<span class="ml-auto text-[10px] font-mono text-text-secondary">#{outgoingMessage.id}</span>
				</div>
				<div class="p-5">
					<p class="text-xs text-text-secondary mb-1">
						<span class="text-text-primary font-mono">@{run.agent_name}</span>
						{#if outgoingMessage.to_agent} → <a href="/agents/{outgoingMessage.to_agent}" class="text-text-link hover:underline font-mono">@{outgoingMessage.to_agent}</a>{/if}
						&middot; {outgoingMessage.status}
					</p>
					<pre class="text-sm text-text-primary bg-bg-primary border border-border rounded-lg p-3 whitespace-pre-wrap font-mono leading-relaxed max-h-80 overflow-y-auto">{outgoingMessage.body}</pre>
				</div>
			</div>
		{/if}

		<!-- Raw run / harness metadata -->
		<div class="card">
			<div class="px-5 py-3 border-b border-border">
				<h2 class="font-semibold text-sm text-text-primary font-display">Metadata</h2>
			</div>
			<div class="p-5 grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs font-mono">
				<div><span class="text-text-secondary">reactive_run.id</span> <span class="text-text-primary">{run.id}</span></div>
				{#if harnessRun}
					<div><span class="text-text-secondary">harness_run.run_id</span> <span class="text-text-primary break-all">{harnessRun.run_id}</span></div>
					<div><span class="text-text-secondary">backend</span> <span class="text-text-primary">{harnessRun.backend}</span></div>
					{#if harnessRun.session_id}
						<div><span class="text-text-secondary">session_id</span> <span class="text-text-primary break-all">{harnessRun.session_id}</span></div>
					{/if}
					{#if harnessRun.tokens_cached > 0}
						<div><span class="text-text-secondary">tokens_cached</span> <span class="text-text-primary">{formatTokens(harnessRun.tokens_cached)}</span></div>
					{/if}
				{/if}
				{#if run.k8s_job_name}
					<div><span class="text-text-secondary">k8s_job</span> <span class="text-text-primary break-all">{run.k8s_job_name}</span></div>
				{/if}
				{#if agent}
					<div><span class="text-text-secondary">agent.harness_name</span> <span class="text-text-primary">{agent.harness_name || '—'}</span></div>
					<div><span class="text-text-secondary">max_trigger_depth</span> <span class="text-text-primary">{agent.max_trigger_depth}</span></div>
					<div><span class="text-text-secondary">daily_budget</span> <span class="text-text-primary">{agent.daily_trigger_budget}</span></div>
					<div><span class="text-text-secondary">cooldown_s</span> <span class="text-text-primary">{agent.cooldown_seconds}</span></div>
				{/if}
			</div>
		</div>
	{/if}
</div>
