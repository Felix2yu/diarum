<script lang="ts">
  let { tagline = '' }: { tagline?: string } = $props();
</script>

<footer class="safe-bottom mt-auto border-t border-border/50">
  <div class="container-responsive py-4">
    <div
      class="flex flex-row flex-wrap items-center justify-center gap-2 sm:justify-between sm:gap-4"
    >
      <div
        class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-xs text-muted-foreground/80"
      >
        {#if tagline}
          <span class="whitespace-nowrap">{tagline}</span>
        {/if}
        <span class="whitespace-nowrap">© {new Date().getFullYear()} 吾身</span>
      </div>
    </div>
  </div>
</footer>
