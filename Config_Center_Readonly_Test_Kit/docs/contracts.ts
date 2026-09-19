/** Proposed production contract. No provider implementations are claimed here. */
export type ConfigKind = 'chat' | 'image' | 'audio' | 'video' | 'database' | 'storage' | 'media';
export type Delivery = 'stream' | 'complete';
export type ContentFormat = 'text' | 'json';
export type RunState = 'submitting' | 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';

export interface CapabilitySet {
  verified: boolean;
  delivery?: Delivery[];
  formats?: ContentFormat[];
  referenceImage?: boolean;
  cancellation: 'supported' | 'unsupported' | 'unknown';
  acceptedInputMimeTypes?: string[];
  maxInputBytes?: number;
}
export interface PublicConfig {
  id: string;
  groupId: string;
  pageId: string;
  kind: ConfigKind;
  title: string;
  revision: string;
  rendererKey: string; // Resolve only from a local allowlisted renderer registry.
  publicProperties: Array<{ label: string; value: string }>;
  credentialState: 'configured' | 'missing' | 'not_required';
  capabilities: CapabilitySet;
  lastVerification?: { at: string; revision: string; status: 'passed' | 'failed' };
  // No secret, password, token, or private raw provider response here.
}
export type TestInput =
  | { kind: 'chat'; messages: Array<{ role: 'user' | 'assistant'; content: string }> }
  | { kind: 'image' | 'video'; prompt: string; referenceAssetIds?: string[] }
  | { kind: 'audio'; text: string }
  | { kind: 'database'; probeId: 'connect' | 'select_one' | 'ping' }
  | { kind: 'storage' | 'media'; objectKey?: string; prefix?: string };
export interface CreateTestRequest {
  configId: string;
  expectedRevision: string;
  idempotencyKey: string;
  input: TestInput;
  outputPreset?: { delivery: Delivery; format: ContentFormat };
}
export interface ArtifactRef {
  assetId: string;
  mimeType: string;
  filename?: string;
  sizeBytes?: number;
  width?: number;
  height?: number;
  durationMs?: number;
  access: { type: 'same_origin'; path: string } | { type: 'signed_url'; url: string; expiresAt: string };
}
export type OutputBlock =
  | { id: string; kind: 'text'; text: string; complete: boolean }
  | { id: string; kind: 'json'; rawText: string; parsed?: unknown; validJson: boolean; schemaValid?: boolean }
  | { id: string; kind: 'image'; assets: ArtifactRef[] }
  | { id: string; kind: 'audio'; asset: ArtifactRef; transcript?: string }
  | { id: string; kind: 'video'; asset: ArtifactRef; poster?: ArtifactRef }
  | { id: string; kind: 'table'; columns: string[]; rows: Array<Array<string | number | boolean | null>> }
  | { id: string; kind: 'diagnostic'; checks: Array<{ label: string; status: 'passed' | 'failed' | 'not_run'; detail?: string }> };
export interface TestRun {
  id: string;
  configId: string;
  configRevision: string;
  state: RunState;
  stageLabel?: string;
  progressPercent?: number; // Only when measured/actually supplied by upstream.
  outputs: OutputBlock[];
  metrics?: { elapsedMs?: number; firstTokenMs?: number; inputTokens?: number; outputTokens?: number };
  cancellation?: 'requested' | 'confirmed' | 'unsupported';
  error?: { code: string; safeMessage: string; requestId: string };
  createdAt: string;
  updatedAt: string;
}
export type RunEvent =
  | { id: string; type: 'status'; runId: string; state: RunState; stageLabel?: string }
  | { id: string; type: 'text_delta'; runId: string; blockId: string; delta: string }
  | { id: string; type: 'output'; runId: string; block: OutputBlock }
  | { id: string; type: 'done'; run: TestRun }
  | { id: string; type: 'error'; runId: string; code: string; safeMessage: string; requestId: string };

/** Server-only. Never serialize this context to the browser or log it. */
export interface AdapterContext {
  configId: string;
  revision: string;
  secretReference?: string;
  allowedDestinationId: string;
}
export interface ProviderAdapter {
  getCapabilities(ctx: AdapterContext): Promise<CapabilitySet>;
  validateInput(input: TestInput, capabilities: CapabilitySet): void;
  execute(ctx: AdapterContext, request: CreateTestRequest, signal: AbortSignal): AsyncIterable<RunEvent>;
  cancel?: (ctx: AdapterContext, runId: string) => Promise<'confirmed' | 'requested' | 'unsupported'>;
}

export const RESPONSE_PRESETS = {
  stream: { delivery: 'stream', format: 'text' },
  json: { delivery: 'complete', format: 'json' },
  complete: { delivery: 'complete', format: 'text' },
} as const;
