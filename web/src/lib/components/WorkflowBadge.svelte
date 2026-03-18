<script lang="ts">
	let { state }: { state: string } = $props();

	let badgeClass = $derived((() => {
		switch (state) {
			case 'proposed':
				return 'bg-accent-yellow/20 text-accent-yellow';
			case 'approved':
				return 'bg-accent-green text-white';
			case 'in_progress':
				return 'bg-accent-blue text-white';
			case 'rejected':
				return 'bg-accent-red text-white';
			case 'done':
				return 'bg-bg-tertiary text-text-secondary';
			case 'published':
				return 'bg-cyan-500/20 text-cyan-400';
			default:
				return 'bg-bg-tertiary text-text-secondary';
		}
	})());
</script>

<span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium leading-tight {badgeClass}">
	{state}
</span>
