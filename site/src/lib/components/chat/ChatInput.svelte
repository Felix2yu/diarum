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

<form on:submit|preventDefault={handleSubmit} class="flex gap-2 sm:gap-3 items-end">
	<div class="flex-1">
		<textarea
			bind:value={content}
			on:keydown={handleKeydown}
			{placeholder}
			{disabled}
			rows="1"
			class="w-full resize-none rounded-2xl border border-border bg-background
				px-4 text-sm leading-[1.5]
				focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary
				disabled:opacity-50 disabled:cursor-not-allowed
				max-h-[200px] shadow-sm box-border transition-all duration-200"
			style="field-sizing: content; padding-top: 12px; padding-bottom: 12px; min-height: 46px;"
		></textarea>
	</div>

	<div class="flex-shrink-0">
		<button
			type="submit"
			disabled={disabled || !content.trim()}
			title="发送消息"
			class="h-[46px] w-[46px] rounded-2xl
				bg-primary text-primary-foreground
				hover:bg-primary/90 active:scale-95 transition-all duration-150
				disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100
				flex items-center justify-center shadow-sm"
		>
			<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
					d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
			</svg>
		</button>
	</div>
</form>
