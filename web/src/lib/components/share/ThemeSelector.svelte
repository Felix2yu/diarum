<script lang="ts">
  import { themes, type ThemeId } from '#lib/utils/imageExport.js';

  export let selected: ThemeId = 'warm-paper';
  export let onSelect: (theme: ThemeId) => void = () => {};

  const themeList = Object.values(themes);
</script>

<div class="grid grid-cols-2 gap-2">
  {#each themeList as theme (theme.id ?? theme.name)}
    <button
      type="button"
      onclick={() => onSelect(theme.id)}
      class="relative rounded-lg border-2 p-3 text-left transition-all duration-200
				{selected === theme.id
        ? 'border-primary ring-2 ring-primary/20'
        : 'border-border hover:border-primary/50'}"
    >
      <div
        class="mb-2 h-8 w-full rounded"
        style="background-color: {theme.background}; border: 1px solid {theme.border};"
      >
        <div
          class="mt-2 ml-2 h-2 w-3/4 rounded"
          style="background-color: {theme.foreground}; opacity: 0.3;"
        ></div>
        <div
          class="mt-1 ml-2 h-1.5 w-1/2 rounded"
          style="background-color: {theme.mutedForeground}; opacity: 0.3;"
        ></div>
      </div>
      <div class="text-xs font-medium text-foreground">{theme.name}</div>
      {#if selected === theme.id}
        <div class="absolute top-2 right-2">
          <svg class="h-4 w-4 text-primary" fill="currentColor" viewBox="0 0 20 20">
            <path
              fill-rule="evenodd"
              d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
              clip-rule="evenodd"
            />
          </svg>
        </div>
      {/if}
    </button>
  {/each}
</div>
