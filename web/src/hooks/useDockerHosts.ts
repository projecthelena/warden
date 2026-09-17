import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

const API_URL = import.meta.env.VITE_API_URL || "";

export interface DockerHost {
    id: string;
    name: string;
    endpoint: string;
    tlsVerify: boolean;
    hasCaCertificate: boolean;
    hasClientCertificate: boolean;
}

export interface DockerContainer {
    id: string;
    name: string;
    image: string;
    state: string;
    status: string;
    health?: string;
}

export interface DockerHostInput {
    name: string;
    endpoint: string;
    tlsVerify: boolean;
    caCert?: string;
    clientCert?: string;
    clientKey?: string;
}

async function api<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetch(`${API_URL}${path}`, {
        credentials: "include",
        headers: init?.body ? { "Content-Type": "application/json", ...init.headers } : init?.headers,
        ...init,
    });
    if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error || "Docker request failed");
    }
    if (response.status === 204) return undefined as T;
    return response.json();
}

export function useDockerHosts() {
    return useQuery({
        queryKey: ["docker-hosts"],
        queryFn: () => api<DockerHost[]>("/api/docker/hosts"),
    });
}

export function useDockerContainers(hostId?: string) {
    return useQuery({
        queryKey: ["docker-containers", hostId],
        queryFn: () => api<DockerContainer[]>(`/api/docker/hosts/${hostId}/containers`),
        enabled: Boolean(hostId),
        retry: false,
    });
}

export function useCreateDockerHost() {
    const client = useQueryClient();
    return useMutation({
        mutationFn: (input: DockerHostInput) => api<DockerHost>("/api/docker/hosts", { method: "POST", body: JSON.stringify(input) }),
        onSuccess: () => client.invalidateQueries({ queryKey: ["docker-hosts"] }),
    });
}

export function useUpdateDockerHost() {
    const client = useQueryClient();
    return useMutation({
        mutationFn: ({ id, input }: { id: string; input: DockerHostInput }) => api<DockerHost>(`/api/docker/hosts/${id}`, { method: "PUT", body: JSON.stringify(input) }),
        onSuccess: (_, variables) => {
            client.invalidateQueries({ queryKey: ["docker-hosts"] });
            client.invalidateQueries({ queryKey: ["docker-containers", variables.id] });
        },
    });
}

export function useDeleteDockerHost() {
    const client = useQueryClient();
    return useMutation({
        mutationFn: (id: string) => api<void>(`/api/docker/hosts/${id}`, { method: "DELETE" }),
        onSuccess: () => client.invalidateQueries({ queryKey: ["docker-hosts"] }),
    });
}

export function useTestDockerHost() {
    return useMutation({ mutationFn: (id: string) => api<{ ok: true }>(`/api/docker/hosts/${id}/test`, { method: "POST" }) });
}
