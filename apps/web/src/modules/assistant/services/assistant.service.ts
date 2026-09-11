import { apiGet, apiPostFull, apiPutFull } from "@/shared/services/http-client";
import type {
  AssistantAuthorization,
  AssistantAuthorizationInput,
  AssistantChannel,
  AssistantChatInput,
  AssistantChatResult,
  AssistantConfig,
  AssistantConfigInput,
  AssistantRunStat,
  AssistantSimulateInput,
} from "@/modules/assistant/types";

// Endpoint diverifikasi dari
// apps/api/internal/modules/assistant/presentation/handler.go (RegisterRoutes).
export const assistantService = {
  chat: (body: AssistantChatInput) =>
    apiPostFull<AssistantChatResult>("/api/v1/assistant/chat", body),
  simulate: (body: AssistantSimulateInput) =>
    apiPostFull<AssistantChatResult>("/api/v1/assistant/simulate", body),
  getStats: () =>
    apiGet<AssistantRunStat[]>("/api/v1/assistant/runs/stats"),
  getChannel: () => apiGet<AssistantChannel>("/api/v1/assistant/channel"),
  connectChannel: () =>
    apiPostFull<AssistantChannel>("/api/v1/assistant/channel/connect"),
  disconnectChannel: () =>
    apiPostFull<null>("/api/v1/assistant/channel/disconnect"),
  resetChannel: () => apiPostFull<null>("/api/v1/assistant/channel/reset"),
  listAuthorizations: (includeRevoked: boolean) =>
    apiGet<AssistantAuthorization[]>("/api/v1/assistant/authorizations", {
      includeRevoked: includeRevoked ? "true" : "false",
    }),
  createAuthorization: (body: AssistantAuthorizationInput) =>
    apiPostFull<AssistantAuthorization>("/api/v1/assistant/authorizations", body),
  updateAuthorization: (id: number, body: AssistantAuthorizationInput) =>
    apiPutFull<AssistantAuthorization>(
      `/api/v1/assistant/authorizations/${id}`,
      body,
    ),
  revokeAuthorization: (id: number) =>
    apiPostFull<null>(`/api/v1/assistant/authorizations/${id}/revoke`),
  getConfig: () => apiGet<AssistantConfig>("/api/v1/assistant/config"),
  saveConfig: (body: AssistantConfigInput) =>
    apiPutFull<AssistantConfig>("/api/v1/assistant/config", body),
};
