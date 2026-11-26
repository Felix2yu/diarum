import type { Media, UploadProgress } from '#lib/api/client.js';
import {
  uploadImage,
  type CheveretoUploadResult,
  type UploadOptions
} from '#lib/utils/uploadImage.js';

export type UploadJobStatus = 'queued' | 'uploading' | 'success' | 'error' | 'canceled';

export interface UploadJob {
  id: string;
  file: File;
  status: UploadJobStatus;
  progress: number;
  error?: string;
  result?: Media | CheveretoUploadResult;
}

export interface UploadJobHandlers {
  onSuccess?: (job: UploadJob) => void;
  onError?: (job: UploadJob) => void;
  onCancel?: (job: UploadJob) => void;
}

interface InternalJob extends UploadJob {
  options: UploadOptions;
  handlers: UploadJobHandlers;
  controller?: AbortController;
}

type Uploader = (file: File, options: UploadOptions) => Promise<Media | CheveretoUploadResult>;

export interface UploadQueueOptions {
  concurrency?: number;
  uploader?: Uploader;
  onUpdate?: (jobs: UploadJob[]) => void;
}

/**
 * 串行调度器：限制并发数，支持进度、取消与重试。
 * 队列本身不关心 Markdown 占位符，由调用方通过 handlers 处理。
 */
export class UploadQueue {
  private jobs: InternalJob[] = [];
  private listeners = new Set<(jobs: UploadJob[]) => void>();
  private active = 0;
  private destroyed = false;

  private readonly concurrency: number;
  private readonly uploader: Uploader;
  private readonly onUpdate?: (jobs: UploadJob[]) => void;

  constructor(options: UploadQueueOptions = {}) {
    this.concurrency = Math.max(1, options.concurrency ?? 3);
    this.uploader = options.uploader ?? uploadImage;
    this.onUpdate = options.onUpdate;
  }

  enqueue(
    file: File,
    options: UploadOptions,
    handlers: UploadJobHandlers = {},
    id: string = createUploadJobId()
  ): string {
    this.jobs.push({
      id,
      file,
      status: 'queued',
      progress: 0,
      options,
      handlers
    });
    this.notify();
    this.pump();
    return id;
  }

  cancel(id: string): void {
    const job = this.jobs.find((item) => item.id === id);
    if (!job) return;

    if (job.status === 'queued') {
      job.status = 'canceled';
      job.handlers.onCancel?.(job);
      this.notify();
      return;
    }

    if (job.status === 'uploading') {
      job.controller?.abort();
    }
  }

  retry(id: string): void {
    const job = this.jobs.find((item) => item.id === id);
    if (!job || (job.status !== 'error' && job.status !== 'canceled')) return;
    job.status = 'queued';
    job.progress = 0;
    job.error = undefined;
    this.notify();
    this.pump();
  }

  remove(id: string): void {
    const index = this.jobs.findIndex((item) => item.id === id);
    if (index === -1) return;
    const job = this.jobs[index];
    if (job.status === 'uploading') {
      job.controller?.abort();
    }
    this.jobs.splice(index, 1);
    this.notify();
  }

  clearFinished(): void {
    this.jobs = this.jobs.filter((job) => job.status === 'queued' || job.status === 'uploading');
    this.notify();
  }

  getJobs(): UploadJob[] {
    return this.jobs.map(toPublicJob);
  }

  subscribe(listener: (jobs: UploadJob[]) => void): () => void {
    this.listeners.add(listener);
    listener(this.getJobs());
    return () => this.listeners.delete(listener);
  }

  destroy(): void {
    this.destroyed = true;
    for (const job of this.jobs) {
      job.controller?.abort();
    }
    this.listeners.clear();
  }

  private pump(): void {
    if (this.destroyed) return;
    while (this.active < this.concurrency) {
      const next = this.jobs.find((job) => job.status === 'queued');
      if (!next) return;
      void this.start(next);
    }
  }

  private async start(job: InternalJob): Promise<void> {
    if (job.status !== 'queued') return;

    this.active++;
    job.status = 'uploading';
    job.progress = 0;
    job.error = undefined;

    const controller = new AbortController();
    job.controller = controller;
    this.notify();

    try {
      const result = await this.uploader(job.file, {
        ...job.options,
        signal: controller.signal,
        onProgress: (progress: UploadProgress) => {
          job.progress = progress.percentage;
          this.notify();
        }
      });

      const statusAfterUpload = readJobStatus(job);
      if (this.destroyed || statusAfterUpload === 'canceled') return;

      job.status = 'success';
      job.progress = 100;
      job.result = result;
      job.handlers.onSuccess?.(toPublicJob(job));
    } catch (error) {
      if (this.destroyed) return;
      const statusAfterUpload = readJobStatus(job);
      if (statusAfterUpload === 'canceled' || isAbortError(error)) {
        if (statusAfterUpload !== 'canceled') {
          job.status = 'canceled';
          job.handlers.onCancel?.(toPublicJob(job));
        }
      } else {
        job.status = 'error';
        job.error = error instanceof Error ? error.message : '上传失败';
        job.handlers.onError?.(toPublicJob(job));
      }
    } finally {
      job.controller = undefined;
      this.active = Math.max(0, this.active - 1);
      this.notify();
      this.pump();
    }
  }

  private notify(): void {
    const snapshot = this.getJobs();
    this.onUpdate?.(snapshot);
    for (const listener of this.listeners) {
      listener(snapshot);
    }
  }
}

function readJobStatus(job: InternalJob): UploadJobStatus {
  return job.status;
}

function toPublicJob(job: InternalJob): UploadJob {
  return {
    id: job.id,
    file: job.file,
    status: job.status,
    progress: job.progress,
    error: job.error,
    result: job.result
  };
}

export function createUploadJobId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException
    ? error.name === 'AbortError'
    : typeof error === 'object' &&
        error !== null &&
        (error as { name?: string }).name === 'AbortError';
}
