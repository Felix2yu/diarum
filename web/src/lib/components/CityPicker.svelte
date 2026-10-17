<script lang="ts">
  import { pb } from '#lib/api/client.js';
  import type { CityInfo } from '#lib/types/city.js';

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
        const response = await fetch(
          `/api/v1/weather/cities?q=${encodeURIComponent(searchQuery)}`,
          {
            headers: {
              Authorization: `Bearer ${pb.authStore.token}`
            }
          }
        );
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
          const response = await fetch(
            `/api/v1/weather/cities?q=${encodeURIComponent(`${latitude},${longitude}`)}`,
            {
              headers: {
                Authorization: `Bearer ${pb.authStore.token}`
              }
            }
          );
          if (response.ok) {
            const cities = await response.json();
            if (cities.length > 0) {
              // Pass coordinates with city info for weather fetching
              const city = { ...cities[0], lat: latitude, lon: longitude };
              selectCity(city);
            } else {
              alert('无法确定当前位置的城市，请手动选择');
            }
          } else {
            const data = await response.json().catch(() => ({}));
            alert(data.error || '定位服务暂时不可用，请手动选择城市');
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
        class="w-full rounded-lg border border-border/60 bg-muted/30 px-3 py-2 pr-8 text-xs transition-colors placeholder:text-muted-foreground/50 focus:border-primary focus:ring-1 focus:ring-primary/30 focus:outline-none"
      />
      {#if selectedCity && !searchQuery}
        <span class="absolute top-1/2 right-2 -translate-y-1/2 text-xs font-medium text-foreground">
          {selectedCity}
        </span>
      {/if}
      {#if isSearching}
        <span class="absolute top-1/2 right-2 -translate-y-1/2">
          <svg class="h-4 w-4 animate-spin text-muted-foreground" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
        </span>
      {/if}
    </div>
    <button
      onclick={handleGeolocation}
      disabled={isLocating}
      class="rounded-lg border border-border/60 bg-muted/50 p-2 transition-colors hover:bg-muted disabled:opacity-50"
      title="定位当前位置"
    >
      {#if isLocating}
        <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
          ></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
      {:else}
        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"
          ></path>
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"
          ></path>
        </svg>
      {/if}
    </button>
  </div>

  {#if showDropdown && searchResults.length > 0}
    <div
      class="absolute top-full right-0 left-0 z-50 mt-1 max-h-48 overflow-y-auto rounded-lg border border-border/50 bg-card shadow-lg"
    >
      {#each searchResults as city (city.name)}
        <button
          type="button"
          onclick={() => selectCity(city)}
          class="flex w-full items-center justify-between px-3 py-2 text-left text-xs transition-colors hover:bg-muted/50"
        >
          <span class="font-medium">{city.name}</span>
          <span class="text-[10px] text-muted-foreground">
            {[city.province, city.country].filter(Boolean).join(', ')}
          </span>
        </button>
      {/each}
    </div>
  {/if}
</div>
