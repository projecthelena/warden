import { create } from 'zustand';
import { toast } from "@/components/ui/use-toast";

export interface MonitorEvent {
    id: string;
    type: 'up' | 'down' | 'degraded';
    timestamp: string;
    message: string;
}

export interface NotificationChannel {
    id: string;
    type: 'slack' | 'email' | 'discord' | 'webhook';
    name: string;
    config: {
        webhookUrl?: string;
        email?: string;
        channel?: string;
    };
    enabled: boolean;
}

export interface User {
    name: string;
    email: string;
    avatar: string;
    isAuthenticated: boolean;
    timezone?: string;
}

export interface HistoryPoint {
    status: 'up' | 'down' | 'degraded';
    latency: number;
    timestamp: string;
    statusCode: number;
}

export interface Monitor {
    id: string;
    name: string;
    url: string;
    status: 'up' | 'down' | 'degraded';
    latency: number;
    history: HistoryPoint[];
    lastCheck: string;
    events: MonitorEvent[];
}

export interface Group {
    id: string;
    name: string;
    monitors: Monitor[];
}

export interface Incident {
    id: string;
    title: string;
    description: string;
    status: 'investigating' | 'identified' | 'monitoring' | 'resolved' | 'scheduled' | 'in_progress' | 'completed';
    type: 'incident' | 'maintenance';
    severity: 'minor' | 'major' | 'critical';
    startTime: string;
    endTime?: string;
    affectedGroups: string[];
}

interface MonitorStore {
    groups: Group[];
    incidents: Incident[];
    channels: NotificationChannel[];
    user: User | null;
    isAuthChecked: boolean;

    // Actions
    checkAuth: () => Promise<void>;
    login: (username: string, password: string) => Promise<boolean>;
    logout: () => Promise<void>;

    // CRUD
    fetchPublicStatus: () => Promise<void>;
    fetchMonitors: () => Promise<void>;
    addGroup: (name: string) => Promise<void>;
    updateGroup: (id: string, name: string) => Promise<void>;
    deleteGroup: (id: string) => Promise<void>;
    addMonitor: (name: string, url: string, groupName: string) => Promise<void>;
    updateMonitor: (id: string, updates: Partial<Monitor>) => void;
    deleteMonitor: (id: string) => Promise<void>;

    addIncident: (incident: Omit<Incident, 'id' | 'date'>) => void;
    resolveIncident: (id: string) => void;
    addChannel: (channel: Omit<NotificationChannel, 'id'>) => void;
    updateChannel: (id: string, updates: Partial<NotificationChannel>) => void;
    deleteChannel: (id: string) => void;

    updateUser: (data: { password?: string; timezone?: string }) => Promise<void>;

    // Status Pages
    fetchStatusPages: () => Promise<StatusPage[]>;
    toggleStatusPage: (slug: string, publicStatus: boolean, title?: string, groupId?: string) => Promise<void>;
    fetchPublicStatusBySlug: (slug: string) => Promise<any>;

    // API Keys
    fetchAPIKeys: () => Promise<APIKey[]>;
    createAPIKey: (name: string) => Promise<string | null>;
    deleteAPIKey: (id: number) => Promise<void>;
    resetDatabase: () => Promise<boolean>;
}

export interface StatusPage {
    id: number;
    slug: string;
    title: string;
    groupId?: string;
    public: boolean;
    createdAt: string;
}

export interface APIKey {
    id: number;
    keyPrefix: string;
    name: string;
    createdAt: string;
    lastUsed?: string;
    createAPIKey: (name: string) => Promise<string | null>;
    deleteAPIKey: (id: string) => Promise<void>;

    // Settings
    settings: { latency_threshold: string } | null;
    fetchSettings: () => Promise<void>;
    updateSettings: (settings: { latency_threshold: string }) => Promise<void>;
}

export const useMonitorStore = create<MonitorStore>((set, get) => ({
    groups: [],
    incidents: [],
    channels: [],
    user: null,
    isAuthChecked: false,
    settings: null,

    // ... (existing actions)

    updateUser: async (data) => {
        try {
            const res = await fetch("/api/auth/me", {
                method: "PATCH",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(data),
                credentials: 'include'
            });
            if (!res.ok) throw new Error("Failed to update settings");

            // Update local user state if timezone changed
            const currentUser = get().user;
            if (currentUser && data.timezone) {
                set({ user: { ...currentUser, timezone: data.timezone } });
            }
        } catch (error) {
            console.error(error);
            throw error;
        }
    },

    fetchStatusPages: async () => {
        try {
            const res = await fetch("/api/status-pages", { credentials: 'include' });
            if (res.ok) {
                const data = await res.json();
                return data.pages || [];
            }
        } catch (error) {
            console.error(error);
        }
        return [];
    },

    toggleStatusPage: async (slug, publicStatus, title, groupId) => {
        try {
            await fetch(`/api/status-pages/${slug}`, {
                method: "PATCH",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ public: publicStatus, title: title || slug, groupId: groupId }),
                credentials: 'include'
            });
            // Ideally refetch pages here if stored in state, but we return Promise for component to handle
        } catch (error) {
            console.error(error);
            throw error;
        }
    },

    checkAuth: async () => {
        // ...

        try {
            const res = await fetch('/api/auth/me', { credentials: 'include' });
            if (res.ok) {
                const data = await res.json();
                set({
                    user: {
                        name: data.user.username,
                        email: "admin@clusteruptime.com",
                        avatar: data.user.avatar || "https://github.com/shadcn.png",
                        isAuthenticated: true
                    },
                    isAuthChecked: true
                });
                // Once auth is confirmed, fetch private data
                get().fetchMonitors();
            } else {
                set({ user: null, isAuthChecked: true });
            }
        } catch {
            set({ user: null, isAuthChecked: true });
        }
    },

    login: async (username, password) => {
        try {
            const res = await fetch('/api/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password }),
                credentials: 'include'
            });

            if (res.ok) {
                const data = await res.json();
                set({
                    user: {
                        name: data.user.username,
                        email: "admin@clusteruptime.com",
                        avatar: data.user.avatar || "https://github.com/shadcn.png",
                        isAuthenticated: true
                    }
                });
                get().fetchMonitors();
                return true;
            }
        } catch (e) {
            console.error(e);
        }
        return false;
    },

    logout: async () => {
        try {
            await fetch('/api/auth/logout', { method: 'POST', credentials: 'include' });
        } catch (e) {
            console.error(e);
        }
        set({ user: null, groups: [] }); // Clear data on logout
    },

    fetchMonitors: async () => {
        try {
            const res = await fetch('/api/uptime', { credentials: 'include' });
            if (res.ok) {
                const data = await res.json();
                // Backend now returns { groups: [...] }
                set({ groups: data.groups || [] });
            }
        } catch (e) {
            console.error("Failed to fetch monitors", e);
        }
    },



    // ...

    fetchPublicStatusBySlug: async (slug: string) => {
        try {
            const res = await fetch(`/api/s/${slug}`);
            if (res.ok) {
                const data = await res.json();
                // Return data directly or set to a store state?
                // For now, let's return it so component can handle loading state locally if desired, 
                // OR we can adapt 'groups' state.
                // But 'groups' is for the Admin dashboard. 
                // Let's just return the data used by the public view.
                return data;
            }
        } catch (error) {
            console.error(error);
        }
        return null;
    },

    fetchPublicStatus: async () => {
        // Legacy
        try {
            const res = await fetch('/api/status');
            if (res.ok) {
                const data = await res.json();
                if (data.groups) {
                    set({ groups: data.groups });
                }
            }
        } catch (e) {
            console.error("Failed to fetch public status", e);
        }
    },

    // Client-side actions (now API backed)
    addGroup: async (name: string) => {
        try {
            const res = await fetch("/api/groups", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ name }),
            });
            if (!res.ok) {
                // Try to parse conflict error message
                const text = await res.text();
                // Simple heuristic to extract message if JSON
                try {
                    const json = JSON.parse(text);
                    throw new Error(json.error || "Failed to create group");
                } catch {
                    throw new Error(text || "Failed to create group");
                }
            }
            const group = await res.json();
            set((state) => ({ groups: [...(state.groups || []), { ...group, monitors: [] }] }));
            toast({
                title: "Group Created",
                description: `Group "${name}" created successfully.`,
                className: "bg-green-950 border-green-900 text-green-100",
            });
        } catch (error: any) {
            toast({
                title: "Failed to create group",
                description: error.message,
                variant: "destructive",
            });
        }
    },

    updateGroup: async (id: string, name: string) => {
        try {
            const res = await fetch(`/api/groups/${id}`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ name }),
            });

            if (!res.ok) throw new Error("Failed to update group");

            set((state) => ({
                groups: state.groups.map((g) =>
                    g.id === id ? { ...g, name } : g
                ),
            }));

            toast({
                title: "Group Updated",
                description: `Group "${name}" updated successfully.`,
                className: "bg-blue-950 border-blue-900 text-blue-100",
            });
        } catch (error: any) {
            toast({
                title: "Failed to update group",
                description: error.message,
                variant: "destructive",
            });
        }
    },

    deleteGroup: async (id) => {
        try {
            const res = await fetch(`/api/groups/${id}`, {
                method: 'DELETE',
                credentials: 'include'
            });
            if (res.ok) {
                get().fetchMonitors();
                toast({ title: "Group Deleted", description: "Group deleted successfully." });
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to delete group.", variant: "destructive" });
        }
    },

    addMonitor: async (name, url, groupName) => {
        // We need groupID. If groupName is passed, we must find ID or create it?
        // Current UI passes groupName (string). 
        // Backend expects GroupID.
        // Logic: Find group by name. If not found, create it first?
        // Or change UI to pass Group ID.
        // Let's assume UI passes Group ID if it selects from list, or we handle "New Group" logic here.
        // For robustness, let's find the group ID from the name if possible.

        const groups = get().groups;
        let groupId = groups.find(g => g.name === groupName)?.id;

        if (!groupId) {
            // Create group implicitly? Or fail? 
            // Let's create it implicitly to support "New Group" flow from simple UI
            // BUT `addGroup` is async.
            try {
                const res = await fetch('/api/groups', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name: groupName || "Default" }),
                    credentials: 'include'
                });
                if (res.ok) {
                    const newGroup = await res.json();
                    groupId = newGroup.id;
                } else {
                    return; // Fail
                }
            } catch (e) {
                console.error(e);
                return;
            }
        }

        try {
            const res = await fetch('/api/monitors', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, url, groupId }),
                credentials: 'include'
            });
            if (res.ok) {
                get().fetchMonitors();
                toast({ title: "Monitor Created", description: `Monitor "${name}" created successfully.` });
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to create monitor.", variant: "destructive" });
        }
    },

    updateMonitor: (id, updates) => { }, // Placeholder for now

    deleteMonitor: async (id) => {
        try {
            const res = await fetch(`/api/monitors/${id}`, {
                method: 'DELETE',
                credentials: 'include'
            });
            if (res.ok) {
                get().fetchMonitors();
                toast({ title: "Monitor Deleted", description: "Monitor deleted successfully." });
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to delete monitor.", variant: "destructive" });
        }
    },

    addIncident: (incident) => set((state) => ({
        incidents: [{ ...incident, id: Math.random().toString(36).substr(2, 9) }, ...state.incidents]
    })),
    addChannel: (channel) => set((state) => ({
        channels: [...state.channels, { ...channel, id: Math.random().toString(36).substr(2, 9) }]
    })),
    resolveIncident: (id) => set((state) => ({
        incidents: state.incidents.map(inc => inc.id === id ? { ...inc, status: 'resolved' as const } : inc)
    })),
    updateChannel: (id, updates) => set((state) => ({
        channels: state.channels.map(ch => ch.id === id ? { ...ch, ...updates } : ch)
    })),
    deleteChannel: (id) => set((state) => ({
        channels: state.channels.filter(ch => ch.id !== id)
    })),

    fetchAPIKeys: async () => {
        try {
            const res = await fetch("/api/api-keys", { credentials: "include" });
            if (res.ok) {
                const data = await res.json();
                return data.keys || [];
            }
        } catch (error) {
            console.error("Failed to fetch API keys:", error);
        }
        return [];
    },

    createAPIKey: async (name: string) => {
        try {
            const res = await fetch("/api/api-keys", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ name }),
                credentials: "include"
            });
            if (res.ok) {
                const data = await res.json();
                return data.key;
            }
        } catch (error) {
            console.error("Failed to create API key:", error);
        }
        return null;
    },

    deleteAPIKey: async (id: number) => {
        try {
            await fetch(`/api/api-keys/${id}`, {
                method: "DELETE",
                credentials: "include"
            });
        } catch (error) {
            console.error("Failed to delete API key:", error);
        }
    },

    resetDatabase: async () => {
        try {
            const res = await fetch("/api/admin/reset", {
                method: "POST",
                credentials: "include"
            });
            if (res.ok) {
                // Force logout cleanup on frontend
                get().logout();
                return true;
            }
        } catch (error) {
            console.error("Failed to reset database:", error);
        }
        return false;
    },

    fetchSettings: async () => {
        try {
            const res = await fetch('/api/settings', { credentials: 'include' });
            if (res.ok) {
                const settings = await res.json();
                set({ settings });
            }
        } catch (error) {
            console.error('Failed to fetch settings:', error);
        }
    },

    updateSettings: async (newSettings) => {
        try {
            await fetch('/api/settings', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(newSettings),
                credentials: 'include'
            });
            set({ settings: newSettings });
        } catch (error) {
            console.error('Failed to update settings:', error);
        }
    }
}));
