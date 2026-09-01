<script lang="ts">
	interface Props {
		mood: number;
		size?: number;
		class?: string;
	}

	let { mood, size = 24, class: className = '' }: Props = $props();

	const labels: Record<number, string> = {
		1: '非常不愉快',
		2: '不愉快',
		3: '不悲不喜',
		4: '愉快',
		5: '非常愉快'
	};

	// ── 眉（Brow）：5 档梯度（整体上移 0.6，与眼保持间距但不粘连）────────────
	type Vec4 = [string, string, string, string]; // Lx1 Ly1 Lx2 Ly2
	const browPaths: Record<number, { L: Vec4; R: Vec4; sw: number }> = {
		1: { // 深皱八字：内端低、外端高（愤怒/悲伤的"皱紧"）
			L: ['6.2', '6.6', '9.6', '8.0'] as Vec4,
			R: ['17.8', '6.6', '14.4', '8.0'] as Vec4,
			sw: 1.4
		},
		2: { // 微垂：内端略高、外端低（失落/轻微不满）
			L: ['6.6', '7.2', '9.8', '8.0'] as Vec4,
			R: ['17.4', '7.2', '14.2', '8.0'] as Vec4,
			sw: 1.2
		},
		3: { // 平眉
			L: ['6.6', '7.4', '9.8', '7.4'] as Vec4,
			R: ['14.2', '7.4', '17.4', '7.4'] as Vec4,
			sw: 1.1
		},
		4: { // 微挑：内端略低、外端高（精神）
			L: ['6.6', '7.8', '9.8', '7.0'] as Vec4,
			R: ['17.4', '7.8', '14.2', '7.0'] as Vec4,
			sw: 1.2
		},
		5: { // 上挑：外端高高扬起（开心大笑）
			L: ['6.2', '8.4', '9.8', '6.2'] as Vec4,
			R: ['17.8', '8.4', '14.2', '6.2'] as Vec4,
			sw: 1.4
		}
	};

	// ── 眼（Eye）：横向拉开（8.0 / 16.0），1-4 圆眼稍微放大；M5 改成眯眯眼缝型 ──
	//    圆眼 cx 由 8.5/15.5 → 8.0/16.0，cy 保持 10 附近但略下移动到 10.0，
	//    半径从 1.15 → 1.3，更清晰可读。
	//    M5 弯弯笑眼 改成"两端低、中间高"的 下凸 U 型开口向下 = 经典眯眯笑眼，
	//    笔画加粗到 sw=1.9，两端起点外延 (x=6.2/17.8)，与大笑的厚嘴对称。
	type EyeDef =
		| { type: 'dot'; cx: number; cy: number; r: number }
		| { type: 'arc'; d: string; sw: number };
	const eyes: Record<number, { L: EyeDef; R: EyeDef }> = {
		1: { L: { type: 'dot', cx: 8.0, cy: 10.0, r: 1.3 },
			 R: { type: 'dot', cx: 16.0, cy: 10.0, r: 1.3 } },
		2: { L: { type: 'dot', cx: 8.0, cy: 10.0, r: 1.3 },
			 R: { type: 'dot', cx: 16.0, cy: 10.0, r: 1.3 } },
		3: { L: { type: 'dot', cx: 8.0, cy: 10.0, r: 1.3 },
			 R: { type: 'dot', cx: 16.0, cy: 10.0, r: 1.3 } },
		4: { L: { type: 'dot', cx: 8.0, cy:  9.9, r: 1.3 },
			 R: { type: 'dot', cx: 16.0, cy:  9.9, r: 1.3 } },
		5: { // 眯眯笑眼：两端低 y=10.4，中间高高拱起 y=7.6 → 开口向下的眼缝
			L: { type: 'arc', sw: 1.9, d: 'M 6.2 10.4 Q 8.1 7.6 10.0 10.4' },
			R: { type: 'arc', sw: 1.9, d: 'M 14.0 10.4 Q 15.9 7.6 17.8 10.4' }
		}
	};

	// ── 嘴（Mouth）：整体下移 1.2 单位，远离眼睛；横向不变保持原宽度 ─────────
	type MouthDef = { d: string; sw: number };
	const mouthPaths: Record<number, MouthDef> = {
		1: { // 大哭：深下弧 + 顶部两端略下压（嘴角下垂）
			d: 'M 7.5 16.7 Q 12 13.7 16.5 16.7',
			sw: 1.7
		},
		2: { // 小不悦：浅下弧
			d: 'M 8.2 16.4 Q 12 14.4 15.8 16.4',
			sw: 1.55
		},
		3: { // 中性：平直线
			d: 'M 8.8 16.0 L 15.2 16.0',
			sw: 1.5
		},
		4: { // 微笑：浅上弧
			d: 'M 8.2 15.4 Q 12 17.4 15.8 15.4',
			sw: 1.55
		},
		5: { // 大笑：深上弧（夸张的笑）
			d: 'M 7.2 15.2 Q 12 19.2 16.8 15.2',
			sw: 1.7
		}
	};

	// ── 腮红（Blush）：5 档从"暗影→淡→无→淡→粉嫩"连续 ──────────────────────
	//    1 用深灰紫（疲惫/眼圈阴影感）；2 灰；3 极淡中性；4 暖粉；5 粉嫩。
	//    为保持统一 SVG 结构，所有档位都绘制 2 枚腮红圆，仅 opacity 和颜色不同。
	type BlushDef = { opa: number };
	const blushOpacities: Record<number, BlushDef> = {
		1: { opa: 0.22 }, // 阴影感
		2: { opa: 0.14 },
		3: { opa: 0.08 },
		4: { opa: 0.16 },
		5: { opa: 0.30 }  // 粉嫩
	};

	// Mood 1/2 的腮红呈现"冷/阴影"色（表示不开心的疲惫或发青），3-5 为暖粉。
	// 通过 fill 绑定特殊 color：1=var(--mood-shadow，不存在就用#5a5a6a回退)，
	// 简单起见：1、2 档用 fill=face + mood 指定 opa，4、5 档也用 face color，
	// 因为：不开心时 face 深，呈现阴影感；开心时 face 深 + opa 呈现腮红印。
	// 视觉上 4、5 档在暗色模式 face 色会偏浅，这也自然形成"浅腮红"。

	const n = $derived([1, 2, 3, 4, 5].includes(mood) ? mood : 3);
	const label = $derived(labels[n]);
	const brow = $derived(browPaths[n]);
	const eye = $derived(eyes[n]);
	const mouth = $derived(mouthPaths[n]);
	const blush = $derived(blushOpacities[n]);

	// 把 CSS 变量 --mood{n}-bg/face 转发为局部 --m-bg / --m-face，
	// 这样 SVG 内部 var(--m-*) 随全局亮/暗切换自动取色。
	const rootStyle = $derived(
		`--m-bg: var(--mood${n}-bg); --m-face: var(--mood${n}-face);`
	);
</script>

<svg
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	xmlns="http://www.w3.org/2000/svg"
	class={className}
	style={rootStyle}
	role="img"
	aria-label={label}
>
	<!-- ─ 背景外圆 + 淡外环描边（解决浅色系在米白卡片上边界丢失的问题） ─ -->
	<circle
		cx="12"
		cy="12"
		r="11.6"
		fill="var(--m-bg)"
		stroke="var(--m-face)"
		stroke-opacity="0.22"
		stroke-width="0.75"
	/>

	<!-- ─ 内圈淡色晕染：保持原设计"脸部轮廓"的柔和层次感 ─ -->
	<circle
		cx="12"
		cy="12"
		r="8.8"
		fill="var(--m-face)"
		fill-opacity="0.10"
	/>

	<!-- ─ 第 1 维：眉（Brow）─ 2 条，所有档位统一绘制 ────────────────────── -->
	<line
		x1={brow.L[0]} y1={brow.L[1]} x2={brow.L[2]} y2={brow.L[3]}
		stroke="var(--m-face)"
		stroke-width={brow.sw}
		stroke-linecap="round"
	/>
	<line
		x1={brow.R[0]} y1={brow.R[1]} x2={brow.R[2]} y2={brow.R[3]}
		stroke="var(--m-face)"
		stroke-width={brow.sw}
		stroke-linecap="round"
	/>

	<!-- ─ 第 2 维：眼（Eye）─ 2 枚，1-4 圆点，5 弯弧，结构成对一致 ──────── -->
	{#if eye.L.type === 'dot'}
		<circle cx={eye.L.cx} cy={eye.L.cy} r={eye.L.r} fill="var(--m-face)" />
	{:else}
		<path d={eye.L.d} stroke="var(--m-face)" stroke-width={eye.L.sw} fill="none" stroke-linecap="round" />
	{/if}
	{#if eye.R.type === 'dot'}
		<circle cx={eye.R.cx} cy={eye.R.cy} r={eye.R.r} fill="var(--m-face)" />
	{:else}
		<path d={eye.R.d} stroke="var(--m-face)" stroke-width={eye.R.sw} fill="none" stroke-linecap="round" />
	{/if}

	<!-- ─ 第 3 维：嘴（Mouth）─ 5 档连续曲线 ──────────────────────────── -->
	<path
		d={mouth.d}
		stroke="var(--m-face)"
		stroke-width={mouth.sw}
		stroke-linecap="round"
		fill="none"
	/>

	<!-- ─ 第 4 维：腮红（Blush）─ 左右两圆 × 5 档 opacity 连续；cy=14.0 居中于眼-嘴之间 ── -->
	<circle cx="5.8"  cy="14.0" r="1.5" fill="var(--m-face)" fill-opacity={blush.opa} />
	<circle cx="18.2" cy="14.0" r="1.5" fill="var(--m-face)" fill-opacity={blush.opa} />
</svg>
