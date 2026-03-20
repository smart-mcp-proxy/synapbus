<script lang="ts">
	import { onboarding } from '$lib/api/client';

	let skills = $state<any[]>([]);
	let loadingData = $state(true);
	let loadError = $state('');
	let expandedSkill = $state<string | null>(null);
	let skillContent = $state<Record<string, string>>({});
	let loadingContent = $state<Record<string, boolean>>({});

	let _initialized = $state(false);
	$effect(() => {
		if (!_initialized) {
			_initialized = true;
			loadSkills();
		}
	});

	async function loadSkills() {
		loadingData = true;
		loadError = '';
		try {
			const res = await onboarding.skills();
			skills = res.skills || [];
		} catch {
			loadError = 'Skills library is not available yet. The backend endpoint may not be deployed.';
			skills = [];
		} finally {
			loadingData = false;
		}
	}

	function formatSkillName(name: string): string {
		return name
			.replace(/[-_]/g, ' ')
			.replace(/\b\w/g, c => c.toUpperCase());
	}

	function getDescription(skill: any): string {
		return skill.description || 'No description available.';
	}

	async function toggleView(skillName: string) {
		if (expandedSkill === skillName) {
			expandedSkill = null;
			return;
		}
		expandedSkill = skillName;
		if (!skillContent[skillName]) {
			loadingContent = { ...loadingContent, [skillName]: true };
			try {
				const content = await onboarding.skill(skillName);
				skillContent = { ...skillContent, [skillName]: content };
			} catch {
				skillContent = { ...skillContent, [skillName]: 'Failed to load skill content.' };
			} finally {
				loadingContent = { ...loadingContent, [skillName]: false };
			}
		}
	}

	function downloadSkill(skill: any) {
		const content = skillContent[skill.name] || `# ${formatSkillName(skill.name)}\n\n${getDescription(skill)}`;
		const filename = skill.filename || `${skill.name}.md`;
		const blob = new Blob([content], { type: 'text/markdown' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = filename;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		URL.revokeObjectURL(url);
	}

	async function downloadWithFetch(skill: any) {
		// Try to fetch content first if not cached
		if (!skillContent[skill.name]) {
			try {
				const content = await onboarding.skill(skill.name);
				skillContent = { ...skillContent, [skill.name]: content };
			} catch {
				// Use fallback
			}
		}
		downloadSkill(skill);
	}
</script>

<div class="p-5 max-w-5xl">
	<div class="mb-5">
		<h1 class="text-xl font-bold text-text-primary font-display">Skills Library</h1>
		<p class="text-sm text-text-secondary mt-1">Downloadable workflow skills for your agents</p>
	</div>

	{#if loadingData}
		<div class="grid gap-3 sm:grid-cols-2">
			{#each Array(4) as _}
				<div class="card p-4">
					<div class="space-y-2">
						<div class="skeleton h-5 w-1/3"></div>
						<div class="skeleton h-3 w-2/3"></div>
						<div class="skeleton h-3 w-1/2"></div>
					</div>
				</div>
			{/each}
		</div>
	{:else if loadError}
		<div class="card p-8 text-center">
			<svg class="w-10 h-10 mx-auto mb-3 text-text-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
			</svg>
			<p class="text-text-secondary text-sm">{loadError}</p>
		</div>
	{:else if skills.length === 0}
		<div class="card p-8 text-center">
			<svg class="w-10 h-10 mx-auto mb-3 text-text-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25" />
			</svg>
			<p class="text-text-secondary text-sm">No skills available yet.</p>
		</div>
	{:else}
		<div class="grid gap-3 sm:grid-cols-2">
			{#each skills as skill (skill.name)}
				<div class="card">
					<div class="p-4">
						<div class="flex items-start justify-between mb-2">
							<h3 class="font-semibold text-sm text-text-primary font-display">{formatSkillName(skill.name)}</h3>
						</div>
						<p class="text-xs text-text-secondary mb-3 line-clamp-2">{getDescription(skill)}</p>
						<div class="flex gap-2">
							<button
								class="btn-primary text-xs flex items-center gap-1.5"
								onclick={() => downloadWithFetch(skill)}
							>
								<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
								</svg>
								Download
							</button>
							<button
								class="btn-secondary text-xs flex items-center gap-1.5"
								onclick={() => toggleView(skill.name)}
							>
								<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
									<path stroke-linecap="round" stroke-linejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
								</svg>
								{expandedSkill === skill.name ? 'Hide' : 'View'}
							</button>
						</div>
					</div>

					{#if expandedSkill === skill.name}
						<div class="border-t border-border p-4">
							{#if loadingContent[skill.name]}
								<div class="space-y-2">
									<div class="skeleton h-3 w-full"></div>
									<div class="skeleton h-3 w-4/5"></div>
									<div class="skeleton h-3 w-3/5"></div>
								</div>
							{:else}
								<pre class="text-xs font-mono text-text-primary/80 whitespace-pre-wrap break-words max-h-80 overflow-y-auto">{skillContent[skill.name] || 'No content available.'}</pre>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>
