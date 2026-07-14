<script lang="ts">
	import { pb } from '$lib/api/client';
	import type { CityInfo } from '$lib/types/city';

	interface Props {
		selectedCity: string;
		onCitySelect: (city: CityInfo) => void;
	}

	let { selectedCity, onCitySelect }: Props = $props();

	let searchQuery = $state('');
	let searchResults = $state<CityInfo[]>([]);
	let showDropdown = $state(false);
	let isLocating = $state(false);
	let isSearching = $state(false);
	let dropdownRef = $state<HTMLDivElement>();
	let searchTimeout = $state<ReturnType<typeof setTimeout>>();

	async function handleSearch() {
		if (searchTimeout) clearTimeout(searchTimeout);

		if (!searchQuery.trim()) {
			searchResults = [];
			showDropdown = false;
			return;
		}

		// Debounce search
		searchTimeout = setTimeout(async () => {
			isSearching = true;
			try {
				const response = await fetch(`/api/v1/weather/cities?q=${encodeURIComponent(searchQuery)}`, {
					headers: {
						'Authorization': `Bearer ${pb.authStore.token}`
					}
				});
				if (response.ok) {
					searchResults = await response.json();
					showDropdown = searchResults.length > 0;
				}
			} catch (e) {
				console.error('City search failed:', e);
			}
			isSearching = false;
		}, 300);
	}

	function selectCity(city: CityInfo) {
		selectedCity = city.name;
		searchQuery = '';
		searchResults = [];
		showDropdown = false;
		onCitySelect(city);
	}

	async function handleGeolocation() {
		if (!navigator.geolocation) {
			alert('浏览器不支持定位功能');
			return;
		}

		isLocating = true;
		navigator.geolocation.getCurrentPosition(
			async (position) => {
				const { latitude, longitude } = position.coords;
				try {
					// Use backend API to reverse geocode
					const response = await fetch(`/api/v1/weather/cities?q=${encodeURIComponent(`${latitude},${longitude}`)}`, {
						headers: {
							'Authorization': `Bearer ${pb.authStore.token}`
						}
					});
					if (response.ok) {
						const cities = await response.json();
						if (cities.length > 0) {
							selectCity(cities[0]);
						} else {
							alert('无法确定当前位置的城市，请手动选择');
						}
					}
				} catch (e) {
					console.error('Reverse geocoding failed:', e);
					alert('定位失败，请手动选择城市');
				}
				isLocating = false;
			},
			() => {
				alert('定位失败，请手动选择城市');
				isLocating = false;
			},
			{ timeout: 10000 }
		);
	}

	function handleClickOutside(event: MouseEvent) {
		if (dropdownRef && !dropdownRef.contains(event.target as Node)) {
			showDropdown = false;
		}
	}
</script>

<svelte:window onclick={handleClickOutside} />

<div class="relative" bind:this={dropdownRef}>
	<div class="flex items-center gap-2">
		<div class="relative flex-1">
			<input
				type="text"
				bind:value={searchQuery}
				oninput={handleSearch}
				onfocus={() => searchResults.length > 0 && (showDropdown = true)}
				placeholder={selectedCity || '搜索城市...'}
				class="w-full text-xs px-3 py-2 pr-8 rounded-lg bg-muted/30 border border-border/60 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/30 transition-colors placeholder:text-muted-foreground/50"
			/>
			{#if selectedCity && !searchQuery}
				<span class="absolute right-2 top-1/2 -translate-y-1/2 text-xs text-foreground font-medium">
					{selectedCity}
				</span>
			{/if}
			{#if isSearching}
				<span class="absolute right-2 top-1/2 -translate-y-1/2">
					<svg class="w-4 h-4 animate-spin text-muted-foreground" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
				</span>
			{/if}
		</div>
		<button
			onclick={handleGeolocation}
			disabled={isLocating}
			class="p-2 rounded-lg bg-muted/50 hover:bg-muted border border-border/60 transition-colors disabled:opacity-50"
			title="定位当前位置"
		>
			{#if isLocating}
				<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
			{:else}
				<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"></path>
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"></path>
				</svg>
			{/if}
		</button>
	</div>

	{#if showDropdown && searchResults.length > 0}
		<div class="absolute left-0 right-0 top-full mt-1 bg-card border border-border/50 rounded-lg shadow-lg z-50 max-h-48 overflow-y-auto">
			{#each searchResults as city}
				<button
					type="button"
					onclick={() => selectCity(city)}
					class="w-full px-3 py-2 text-left text-xs hover:bg-muted/50 transition-colors flex items-center justify-between"
				>
					<span class="font-medium">{city.name}</span>
					<span class="text-muted-foreground text-[10px]">
						{[city.province, city.country].filter(Boolean).join(', ')}
					</span>
				</button>
			{/each}
		</div>
	{/if}
</div>
