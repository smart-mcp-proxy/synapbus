<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	let { onMenuToggle = () => {} }: { onMenuToggle?: () => void } = $props();

	let searchQuery = $state('');

	function handleSearch(e: SubmitEvent) {
		e.preventDefault();
		if (searchQuery.trim()) {
			goto(`/conversations?q=${encodeURIComponent(searchQuery.trim())}`);
			searchQuery = '';
		}
	}

	let pageTitle = $derived(() => {
		const path = $page.url.pathname;
		if (path === '/') return 'Dashboard';
		if (path.startsWith('/conversations/')) return 'Thread';
		if (path === '/conversations') return 'Search';
		if (path.startsWith('/channels/')) return '#' + ($page.params.name ?? '');
		if (path === '/channels') return 'Channels';
		if (path.startsWith('/dm/')) return ($page.params.name ?? 'DM');
		if (path.startsWith('/agents/')) return ($page.params.name ?? 'Agent');
		if (path === '/agents') return 'Agents';
		if (path === '/settings') return 'Settings';
		return 'SynapBus';
	});
</script>

<header class="flex items-center gap-3 md:gap-4 h-12 px-3 md:px-5 border-b border-border bg-bg-primary flex-shrink-0">
	<!-- Hamburger menu button (mobile only) -->
	<button
		class="md:hidden flex items-center justify-center w-8 h-8 rounded text-text-secondary hover:text-text-primary hover:bg-bg-tertiary transition-colors flex-shrink-0"
		onclick={onMenuToggle}
		aria-label="Toggle sidebar"
	>
		<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
			<path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
		</svg>
	</button>

	<h2 class="font-display font-bold text-text-primary text-base truncate">{pageTitle()}</h2>

	<div class="flex-1"></div>

	<!-- Search -->
	<form class="w-full max-w-xs md:max-w-none md:w-80" onsubmit={handleSearch}>
		<div class="relative">
			<svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
				<path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
			</svg>
			<input
				type="search"
				placeholder="Search messages..."
				class="w-full pl-9 pr-3 py-2 text-sm bg-bg-tertiary border border-border rounded-md text-text-primary placeholder-text-secondary focus:border-border-active focus:ring-0 outline-none"
				bind:value={searchQuery}
			/>
		</div>
	</form>
</header>
