<script lang="ts">
	import type { ShareOptions, ThemeId } from '$lib/utils/imageExport';
	import ThemeSelector from './ThemeSelector.svelte';

	export let options: ShareOptions;
	export let onChange: (options: ShareOptions) => void = () => {};

	function updateOption<K extends keyof ShareOptions>(key: K, value: ShareOptions[K]) {
		options = { ...options, [key]: value };
		onChange(options);
	}

	function handleThemeSelect(theme: ThemeId) {
		updateOption('theme', theme);
	}

	const widthOptions = [
		{ value: 600, label: '600px' },
		{ value: 800, label: '800px' },
		{ value: 1000, label: '1000px' },
		{ value: 1200, label: '1200px' }
	];

	const scaleOptions = [
		{ value: 1, label: '1x' },
		{ value: 2, label: '2x（推荐）' },
		{ value: 3, label: '3x' }
	];
</script>

<div class="space-y-4">
	<!-- Theme Selection -->
	<div>
		<h4 class="text-sm font-medium text-foreground mb-2">主题</h4>
		<ThemeSelector selected={options.theme} onSelect={handleThemeSelect} />
	</div>

	<!-- Display Options -->
	<div>
		<h4 class="text-sm font-medium text-foreground mb-2">显示</h4>
		<div class="space-y-2">
			<label class="flex items-center gap-2 cursor-pointer">
				<input
					type="checkbox"
					checked={options.showDate}
					onchange={(e) => updateOption('showDate', e.currentTarget.checked)}
					class="w-4 h-4 rounded border-border text-primary focus:ring-primary"
				/>
				<span class="text-sm text-foreground">显示日期</span>
			</label>
			<label class="flex items-center gap-2 cursor-pointer">
				<input
					type="checkbox"
					checked={options.showImages}
					onchange={(e) => updateOption('showImages', e.currentTarget.checked)}
					class="w-4 h-4 rounded border-border text-primary focus:ring-primary"
				/>
				<span class="text-sm text-foreground">显示图片</span>
			</label>
			<label class="flex items-center gap-2 cursor-pointer">
				<input
					type="checkbox"
					checked={options.showBranding}
					onchange={(e) => updateOption('showBranding', e.currentTarget.checked)}
					class="w-4 h-4 rounded border-border text-primary focus:ring-primary"
				/>
				<span class="text-sm text-foreground">显示吾身品牌标识</span>
			</label>
		</div>
	</div>

	<!-- Size Options -->
	<div class="grid grid-cols-2 gap-4">
		<div>
			<h4 class="text-sm font-medium text-foreground mb-2">宽度</h4>
			<select
				value={options.width}
				onchange={(e) => updateOption('width', parseInt(e.currentTarget.value))}
				class="w-full px-3 py-2 text-sm bg-background border border-border rounded-lg focus:ring-2 focus:ring-primary/20 focus:border-primary"
			>
				{#each widthOptions as opt}
					<option value={opt.value}>{opt.label}</option>
				{/each}
			</select>
		</div>
		<div>
			<h4 class="text-sm font-medium text-foreground mb-2">质量</h4>
			<select
				value={options.scale}
				onchange={(e) => updateOption('scale', parseInt(e.currentTarget.value))}
				class="w-full px-3 py-2 text-sm bg-background border border-border rounded-lg focus:ring-2 focus:ring-primary/20 focus:border-primary"
			>
				{#each scaleOptions as opt}
					<option value={opt.value}>{opt.label}</option>
				{/each}
			</select>
		</div>
	</div>
</div>
