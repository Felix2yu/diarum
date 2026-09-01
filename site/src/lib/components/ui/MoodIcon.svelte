<script lang="ts">
	interface Props {
		mood: number;
		size?: number;
		class?: string;
	}

	let { mood, size = 24, class: className = '' }: Props = $props();

	const moodConfigs: Record<number, { bg: string; face: string; label: string }> = {
		1: {
			bg: '#FCA5A5',
			face: '#991B1B',
			label: '非常不愉快'
		},
		2: {
			bg: '#FDBA74',
			face: '#9A3412',
			label: '不愉快'
		},
		3: {
			bg: '#FDE047',
			face: '#854D0E',
			label: '不悲不喜'
		},
		4: {
			bg: '#93C5FD',
			face: '#1E40AF',
			label: '愉快'
		},
		5: {
			bg: '#86EFAC',
			face: '#166534',
			label: '非常愉快'
		}
	};

	let config = $derived(moodConfigs[mood] || moodConfigs[3]);
</script>

<svg
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	xmlns="http://www.w3.org/2000/svg"
	class={className}
	role="img"
	aria-label={config.label}
>
	<!-- 背景圆 -->
	<circle cx="12" cy="12" r="12" fill={config.bg} />

	<!-- 脸部轮廓 -->
	<circle cx="12" cy="12" r="9" fill={config.face} fill-opacity="0.15" />

	{#if mood === 1}
		<!-- 非常不愉快：皱眉 + 向下的眼睛 -->
		<circle cx="8.5" cy="10" r="1.2" fill={config.face} />
		<circle cx="15.5" cy="10" r="1.2" fill={config.face} />
		<!-- 皱眉 -->
		<path d="M8 15.5C9 14.5 15 14.5 16 15.5" stroke={config.face} stroke-width="1.5" stroke-linecap="round" />
		<!-- 眉毛下垂 -->
		<path d="M6.5 8L9 9" stroke={config.face} stroke-width="1.2" stroke-linecap="round" />
		<path d="M17.5 8L15 9" stroke={config.face} stroke-width="1.2" stroke-linecap="round" />
	{:else if mood === 2}
		<!-- 不愉快：轻微皱眉 -->
		<circle cx="8.5" cy="10.5" r="1.2" fill={config.face} />
		<circle cx="15.5" cy="10.5" r="1.2" fill={config.face} />
		<!-- 微微向下的嘴 -->
		<path d="M9 15C10 14.5 14 14.5 15 15" stroke={config.face} stroke-width="1.5" stroke-linecap="round" />
	{:else if mood === 3}
		<!-- 中性：平嘴 -->
		<circle cx="8.5" cy="10.5" r="1.2" fill={config.face} />
		<circle cx="15.5" cy="10.5" r="1.2" fill={config.face} />
		<!-- 平嘴 -->
		<line x1="9" y1="14.5" x2="15" y2="14.5" stroke={config.face} stroke-width="1.5" stroke-linecap="round" />
	{:else if mood === 4}
		<!-- 愉快：微笑 -->
		<circle cx="8.5" cy="10.5" r="1.2" fill={config.face} />
		<circle cx="15.5" cy="10.5" r="1.2" fill={config.face} />
		<!-- 微笑 -->
		<path d="M8 14C9.5 15.5 14.5 15.5 16 14" stroke={config.face} stroke-width="1.5" stroke-linecap="round" />
	{:else if mood === 5}
		<!-- 非常愉快：大笑 -->
		<!-- 开心的眼睛（弯弯的） -->
		<path d="M7 10C7.5 9 9 9 9.5 10" stroke={config.face} stroke-width="1.5" stroke-linecap="round" />
		<path d="M14.5 10C15 9 16.5 9 17 10" stroke={config.face} stroke-width="1.5" stroke-linecap="round" />
		<!-- 大笑的嘴 -->
		<path d="M7.5 13.5C9 16 15 16 16.5 13.5" stroke={config.face} stroke-width="1.5" stroke-linecap="round" />
		<!-- 腮红 -->
		<circle cx="6" cy="12.5" r="1.5" fill={config.face} fill-opacity="0.2" />
		<circle cx="18" cy="12.5" r="1.5" fill={config.face} fill-opacity="0.2" />
	{/if}
</svg>
