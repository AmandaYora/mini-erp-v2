// Tipe respons backend modul asisten — diverifikasi dari handler
// apps/api/internal/modules/assistant/presentation/handler.go.
// Semua key camelCase; envelope {success, message, data} sudah dikupas http-client.

export interface AssistantChatResult {
  runId: number;
  branchId: number;
  branchName: string;
  intent: string;
  mode: string;
  answer: string;
}

export interface AssistantChatInput {
  message: string;
  branchId?: number;
}

export interface AssistantSimulateInput {
  phone: string;
  message: string;
  branchId?: number;
}

export interface AssistantChannel {
  connected: boolean;
  phone: string;
  qrDataUrl: string;
  lastError: string;
  updatedAt: string;
}

export type AssistantAccessLevel = "owner" | "authorized_party";

export interface AssistantAuthorization {
  id: number;
  phone: string;
  name: string;
  accessLevel: AssistantAccessLevel;
  status: string;
  isPrimaryOwner: boolean;
  lastSeenAt: string;
}

export interface AssistantAuthorizationInput {
  phone: string;
  name: string;
  accessLevel: AssistantAccessLevel;
  isPrimaryOwner: boolean;
}

export interface AssistantConfig {
  mode: string;
  effectiveMode: string;
  rateLimitPerMinute: number;
  updatedAt: string;
}

export interface AssistantConfigInput {
  mode: "rule_based";
  rateLimitPerMinute: number;
}

export interface AssistantRunStat {
  day: string;
  intent: string;
  mode: string;
  total: number;
  success: number;
  avgDurationMs: number;
}
