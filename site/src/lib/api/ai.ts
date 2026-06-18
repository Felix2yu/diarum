import { pb } from './client';

export interface AISettings {
	api_key: string;
	base_url: string;
	chat_model: string;
	embedding_model: string;
	analysis_system_prompt: string;
	analysis_user_prefix: string;
	enabled: boolean;
}

export interface ModelInfo {
	id: string;
	object: string;
	created?: number;
	owned_by?: string;
}

export interface BuildVectorsResult {
	success: number;
	failed: number;
	total: number;
	errors?: string[];
	error_details?: string[];
}

export interface VectorStats {
	diary_count: number;
	indexed_count: number;
	outdated_count: number;
	pending_count: number;
}

export interface PeriodAnalysisResult {
	id?: string;
	period: 'week' | 'month';
	start: string;
	end: string;
	count: number;
	summary: string;
	system_prompt?: string;
	user_prefix?: string;
	created?: string;
	updated?: string;
}

export interface SavedPeriodAnalysisResult extends PeriodAnalysisResult {
	id: string;
}

/**
 * Retrieve a previously saved AI analysis for a period.
 */
export async function getSavedAnalysis(
	period: 'week' | 'month',
	start: string,
	end: string
): Promise<SavedPeriodAnalysisResult | null> {
	const params = new URLSearchParams({ period, start, end });
	const response = await fetch(`/api/v1/ai/analysis?${params.toString()}`, {
		headers: {
			'Authorization': `Bearer ${pb.authStore.token}`
		}
	});

	if (!response.ok) {
		let message = '获取分析失败';
		try {
			const data = await response.json();
			if (data?.message) message = data.message;
		} catch {
			// ignore
		}
		throw new Error(message);
	}

	const data = await response.json();
	if (!data || data.found === false) return null;
	return data as SavedPeriodAnalysisResult;
}

/**
 * List saved period analyses, optionally filtered by period.
 */
export async function getSavedAnalyses(
	period?: 'week' | 'month' | 'all'
): Promise<SavedPeriodAnalysisResult[]> {
	const params = new URLSearchParams();
	if (period) params.set('period', period);
	const qs = params.toString();
	const response = await fetch(`/api/v1/ai/analyses${qs ? `?${qs}` : ''}`, {
		headers: {
			'Authorization': `Bearer ${pb.authStore.token}`
		}
	});

	if (!response.ok) {
		let message = '获取历史分析列表失败';
		try {
			const data = await response.json();
			if (data?.message) message = data.message;
		} catch {
			// ignore
		}
		throw new Error(message);
	}

	const data = await response.json();
	return (data?.items ?? []) as SavedPeriodAnalysisResult[];
}

export const DEFAULT_ANALYSIS_SYSTEM_PROMPT =
	'你是一个贴心的日记分析助手，基于用户提供的日记内容进行深入分析。你需要：\n1) 归纳总结日记的主要内容；\n2) 分析用户的情绪变化、生活模式；\n3) 找出亮点和值得改进的地方；\n4) 给出具体、可操作的建议。\n请用温暖、鼓励且理性的语气，分段输出，便于阅读。使用中文回答。';

/**
 * Request period (week / month) analysis for a given date range.
 * Optional `system_prompt` and `user_prefix` override saved settings.
 */
export async function analyzePeriod(
	period: 'week' | 'month',
	start: string,
	end: string,
	opts?: { system_prompt?: string; user_prefix?: string }
): Promise<PeriodAnalysisResult> {
	const payload: Record<string, unknown> = { period, start, end };
	if (opts?.system_prompt !== undefined) payload.system_prompt = opts.system_prompt;
	if (opts?.user_prefix !== undefined) payload.user_prefix = opts.user_prefix;

	const response = await fetch('/api/v1/ai/analysis', {
		method: 'POST',
		headers: {
			'Authorization': `Bearer ${pb.authStore.token}`,
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(payload)
	});

	if (!response.ok) {
		let message = 'AI 分析失败';
		try {
			const data = await response.json();
			if (data && data.message) {
				message = data.message;
			}
		} catch {
			// ignore
		}
		throw new Error(message);
	}

	return await response.json();
}

/**
 * Get AI settings
 */
export async function getAISettings(): Promise<AISettings> {
	try {
		const response = await fetch('/api/v1/ai/settings', {
			headers: {
				'Authorization': `Bearer ${pb.authStore.token}`
			}
		});

		if (!response.ok) {
			throw new Error('Failed to get AI settings');
		}

		return await response.json();
	} catch (error) {
		console.error('Error fetching AI settings:', error);
		return {
			api_key: '',
			base_url: '',
			chat_model: '',
			embedding_model: '',
			enabled: false
		};
	}
}

/**
 * Save AI settings
 */
export async function saveAISettings(settings: AISettings): Promise<{ success: boolean }> {
	const response = await fetch('/api/v1/ai/settings', {
		method: 'PUT',
		headers: {
			'Authorization': `Bearer ${pb.authStore.token}`,
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(settings)
	});

	if (!response.ok) {
		const data = await response.json();
		throw new Error(data.message || 'Failed to save AI settings');
	}

	return await response.json();
}

/**
 * Fetch available models from OpenAI-compatible API
 */
export async function fetchModels(apiKey: string, baseUrl: string): Promise<ModelInfo[]> {
	const response = await fetch('/api/v1/ai/models', {
		method: 'POST',
		headers: {
			'Authorization': `Bearer ${pb.authStore.token}`,
			'Content-Type': 'application/json'
		},
		body: JSON.stringify({
			api_key: apiKey,
			base_url: baseUrl
		})
	});

	if (!response.ok) {
		const data = await response.json();
		throw new Error(data.message || 'Failed to fetch models');
	}

	const data = await response.json();
	return data.models || [];
}

/**
 * Build vectors for all diaries (full rebuild)
 */
export async function buildVectors(): Promise<BuildVectorsResult> {
	const response = await fetch('/api/v1/ai/vectors/build', {
		method: 'POST',
		headers: {
			'Authorization': `Bearer ${pb.authStore.token}`,
			'Content-Type': 'application/json'
		}
	});

	if (!response.ok) {
		const data = await response.json();
		throw new Error(data.message || 'Failed to build vectors');
	}

	return await response.json();
}

/**
 * Build vectors incrementally (only new and outdated)
 */
export async function buildVectorsIncremental(): Promise<BuildVectorsResult> {
	const response = await fetch('/api/v1/ai/vectors/build-incremental', {
		method: 'POST',
		headers: {
			'Authorization': `Bearer ${pb.authStore.token}`,
			'Content-Type': 'application/json'
		}
	});

	if (!response.ok) {
		const data = await response.json();
		throw new Error(data.message || 'Failed to build vectors');
	}

	return await response.json();
}

/**
 * Get vector stats
 */
export async function getVectorStats(): Promise<VectorStats> {
	const response = await fetch('/api/v1/ai/vectors/stats', {
		headers: {
			'Authorization': `Bearer ${pb.authStore.token}`
		}
	});

	if (!response.ok) {
		const data = await response.json();
		throw new Error(data.message || 'Failed to get vector stats');
	}

	return await response.json();
}

export interface PolishResult {
	content: string;
	mode: string;
}

/**
 * Polish / rewrite diary text using AI.
 * mode: "medium" (去语气词纠错分段) | "strong" (重组精简) | "custom" (自定义 prompt)
 */
export async function polishText(
	content: string,
	mode: 'medium' | 'strong' | 'custom',
	customPrompt?: string
): Promise<PolishResult> {
	const payload: Record<string, unknown> = {
		content,
		mode
	};
	if (customPrompt !== undefined) {
		payload.prompt = customPrompt;
	}

	const response = await fetch('/api/v1/ai/polish', {
		method: 'POST',
		headers: {
			'Authorization': `Bearer ${pb.authStore.token}`,
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(payload)
	});

	if (!response.ok) {
		let message = 'AI 整理失败';
		try {
			const data = await response.json();
			if (data?.message) message = data.message;
		} catch {
			// ignore
		}
		throw new Error(message);
	}

	return await response.json();
}
