<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let disabled = false;
  export let placeholder = '输入您的消息...';

  let content = '';
  const dispatch = createEventDispatcher<{ send: string }>();

  function handleSubmit() {
    if (content.trim() && !disabled) {
      dispatch('send', content.trim());
      content = '';
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  }
</script>

<form
  onsubmit={(e) => {
    e.preventDefault();
    handleSubmit();
  }}
  class="flex items-end gap-2 sm:gap-3"
>
  <div class="flex-1">
    <textarea
      bind:value={content}
      onkeydown={handleKeydown}
      {placeholder}
      {disabled}
      rows="1"
      class="box-border max-h-[200px] w-full resize-none rounded-2xl border
				border-border bg-background px-4
				text-sm leading-[1.5] shadow-sm transition-all
				duration-200 focus:border-primary
				focus:ring-2 focus:ring-primary/50 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      style="field-sizing: content; padding-top: 12px; padding-bottom: 12px; min-height: 46px;"
    ></textarea>
  </div>

  <div class="flex-shrink-0">
    <button
      type="submit"
      disabled={disabled || !content.trim()}
      title="发送消息"
      class="flex h-[46px] w-[46px]
				items-center justify-center
				rounded-2xl bg-primary text-primary-foreground shadow-sm
				transition-all duration-150 hover:bg-primary/90
				active:scale-95 disabled:cursor-not-allowed disabled:opacity-50 disabled:active:scale-100"
    >
      <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"
        />
      </svg>
    </button>
  </div>
</form>
