import type { Environment } from "./utils";

const API_PREFIX = "/api";

const normalizeEnvironment = (value?: string): Environment => {
  switch ((value || "").toLowerCase()) {
    case "production":
    case "prod":
      return "production";
    case "preprod":
    case "staging":
      return "preprod";
    case "system":
      return "system";
    case "nonprod":
    case "sandbox":
    case "dev":
    case "development":
      return "development";
    case "unknown":
    default:
      return "unknown";
  }
};

type OverviewResponseApi = {
  clusterName: string;
  timestamp: string;
  totalHourlyCost: number;
  totalMonthlyCost: number;
  envCostHourly: Record<string, number>;
  topNamespacesByCost: Array<{
    namespace: string;
    environment: string;
    hourlyCost: number;
  }>;
  savingsCandidates: Array<{
    namespace: string;
    environment: string;
    hourlyCost: number;
    cpuRequestMilli: number;
    cpuUsageMilli: number;
    memoryRequestBytes: number;
    memoryUsageBytes: number;
  }>;
};

export type OverviewResponse = {
  clusterName: string;
  timestamp: string;
  totalHourlyCost: number;
  totalMonthlyCost: number;
  envCostHourly: Record<string, number>;
  topNamespacesByCost: Array<{
    namespace: string;
    environment: Environment;
    hourlyCost: number;
  }>;
  savingsCandidates: Array<{
    namespace: string;
    environment: Environment;
    hourlyCost: number;
    cpuRequestMilli: number;
    cpuUsageMilli: number;
    memoryRequestBytes: number;
    memoryUsageBytes: number;
  }>;
};

type NamespaceCostRecordApi = {
  namespace: string;
  hourlyCost: number;
  podCount: number;
  cpuRequestMilli: number;
  cpuUsageMilli: number;
  memoryRequestBytes: number;
  memoryUsageBytes: number;
  labels?: Record<string, string>;
  environment: string;
};

type NamespaceListApiResponse = {
  items: NamespaceCostRecordApi[];
  totalCount: number;
  timestamp: string;
};

export interface NamespaceCostRecord {
  namespace: string;
  hourlyCost: number;
  podCount: number;
  cpuRequestMilli: number;
  cpuUsageMilli: number;
  memoryRequestBytes: number;
  memoryUsageBytes: number;
  labels: Record<string, string>;
  environment: Environment;
}

export interface NamespacesResponse {
  lastUpdated: string;
  totalCount: number;
  records: NamespaceCostRecord[];
}

type NodeCostApi = {
  nodeName: string;
  hourlyCost: number;
  cpuUsagePercent: number;
  memoryUsagePercent: number;
  cpuAllocatableMilli?: number;
  memoryAllocatableBytes?: number;
  podCount: number;
  status: "Ready" | "NotReady" | "Unknown";
  isUnderPressure: boolean;
  instanceType?: string;
  labels?: Record<string, string>;
  taints?: string[];
};

type NodeListApiResponse = {
  items: NodeCostApi[];
  totalCount: number;
  timestamp: string;
};

export interface NodeCost extends NodeCostApi {
  lastUpdated?: string;
}

type ResourcesApiResponse = {
  timestamp: string;
  cpu: {
    usageMilli: number;
    requestMilli: number;
    efficiencyPercent: number;
    estimatedHourlyWasteCost: number;
  };
  memory: {
    usageBytes: number;
    requestBytes: number;
    efficiencyPercent: number;
    estimatedHourlyWasteCost: number;
  };
  namespaceWaste: Array<{
    namespace: string;
    environment: string;
    cpuWastePercent: number;
    memoryWastePercent: number;
    estimatedHourlyWasteCost: number;
  }>;
};

export type ResourcesSummary = {
  timestamp: string;
  cpu: {
    usageMilli: number;
    requestMilli: number;
    efficiencyPercent: number;
    estimatedHourlyWasteCost: number;
  };
  memory: {
    usageBytes: number;
    requestBytes: number;
    efficiencyPercent: number;
    estimatedHourlyWasteCost: number;
  };
  namespaceWaste: Array<{
    namespace: string;
    environment: Environment;
    cpuWastePercent: number;
    memoryWastePercent: number;
    estimatedHourlyWasteCost: number;
  }>;
};

export type AgentDatasetStatus = "ok" | "partial" | "missing";

export interface AgentStatusResponse {
  status: "connected" | "partial" | "offline";
  lastSync: string;
  datasets: {
    namespaces: AgentDatasetStatus;
    nodes: AgentDatasetStatus;
    resources: AgentDatasetStatus;
  };
  version?: string;
  updateAvailable: boolean;
  clusterName?: string;
  clusterType?: string;
  clusterRegion?: string;
  region?: string;
  nodeCount?: number;
}

export interface HealthResponse {
  status: string;
  clusterId?: string;
  clusterName?: string;
  clusterType?: string;
  clusterRegion?: string;
  version?: string;
  timestamp: string;
}

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${API_PREFIX}${path}`);
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Request failed with ${response.status}`);
  }
  return response.json();
}

export const fetchOverview = async (): Promise<OverviewResponse> => {
  const resp = await request<OverviewResponseApi>("/cost/overview");
  return {
    ...resp,
    topNamespacesByCost: resp.topNamespacesByCost.map((item) => ({
      ...item,
      environment: normalizeEnvironment(item.environment)
    })),
    savingsCandidates: resp.savingsCandidates.map((item) => ({
      ...item,
      environment: normalizeEnvironment(item.environment)
    }))
  };
};

export const fetchNamespaces = async (): Promise<NamespacesResponse> => {
  const resp = await request<NamespaceListApiResponse>("/cost/namespaces");
  return {
    lastUpdated: resp.timestamp,
    totalCount: resp.totalCount,
    records: resp.items.map((record) => ({
      ...record,
      labels: record.labels ?? {},
      environment: normalizeEnvironment(record.environment)
    }))
  };
};

export const fetchNodes = async (): Promise<NodeCost[]> => {
  const resp = await request<NodeListApiResponse>("/cost/nodes");
  return resp.items.map((node) => ({
    ...node,
    labels: node.labels ?? {},
    taints: node.taints ?? [],
    lastUpdated: resp.timestamp
  }));
};

export const fetchResources = async (): Promise<ResourcesSummary> => {
  const resp = await request<ResourcesApiResponse>("/cost/resources");
  return {
    timestamp: resp.timestamp,
    cpu: resp.cpu,
    memory: resp.memory,
    namespaceWaste: resp.namespaceWaste.map((entry) => ({
      ...entry,
      environment: normalizeEnvironment(entry.environment)
    }))
  };
};

export const fetchHealth = () => request<HealthResponse>("/health");
export const fetchAgentStatus = () => request<AgentStatusResponse>("/agent");
