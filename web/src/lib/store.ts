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
    interval: number;
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



export interface OverviewGroup {
    id: string;
    name: string;
    status: 'up' | 'down' | 'degraded' | 'maintenance';
}

export interface Settings {
    latency_threshold: string;
    data_retention_days: string;
}

export interface StatusPage {
    id: number;
    slug: string;
    title: string;
    groupId?: string;
    public: boolean;
    createdAt: string;
}

export interface SystemIncident {
    id: string;
    monitorId: string;
    monitorName: string;
    groupName: string;
    groupId: string;
    type: 'down' | 'degraded';
    message: string;
    startedAt: string;
    resolvedAt?: string;
    duration: string;
}

export interface APIKey {
    id: number;
    keyPrefix: string;
    name: string;
    createdAt: string;
    lastUsed?: string;
}

export interface SystemStats {
    version: string;
    dbSize: number;
    stats: {
        totalMonitors: number;
        activeMonitors: number;
        downMonitors: number;
        degradedMonitors: number;
        totalGroups: number;
        dailyPingsEstimate: number;
    };
}

interface MonitorStore {
    groups: Group[];
    overview: OverviewGroup[];
    incidents: Incident[]; // Manual incidents
    systemEvents: { active: SystemIncident[], history: SystemIncident[] }; // Auto events
    channels: NotificationChannel[];
    user: User | null;
    isAuthChecked: boolean;
    isSetupComplete: boolean;

    apiKeys: APIKey[];

    // Actions
    checkAuth: () => Promise<void>;
    login: (username: string, password: string) => Promise<{ success: boolean; error?: string }>;
    logout: () => Promise<void>;

    // CRUD
    fetchPublicStatus: () => Promise<void>;
    fetchOverview: () => Promise<void>;
    fetchMonitors: (groupId?: string) => Promise<void>;
    setGroups: (groups: Group[]) => void; // For React Query Sync
    setSystemEvents: (events: { active: SystemIncident[], history: SystemIncident[] }) => void;
    fetchSystemEvents: () => Promise<void>;
    addGroup: (name: string) => Promise<string | undefined>;
    updateGroup: (id: string, name: string) => Promise<void>;
    deleteGroup: (id: string) => Promise<void>;
    addMonitor: (name: string, url: string, groupName: string, interval?: number) => Promise<void>;
    updateMonitor: (id: string, updates: Partial<Monitor>) => void;
    deleteMonitor: (id: string) => Promise<void>;

    addIncident: (incident: Omit<Incident, 'id' | 'createdAt' | 'type'>) => void;
    addMaintenance: (maintenance: Omit<Incident, 'id' | 'createdAt' | 'type' | 'severity'>) => void;
    resolveIncident: (id: string) => void;
    fetchIncidents: () => Promise<void>;
    addChannel: (channel: Omit<NotificationChannel, 'id' | 'enabled'>) => Promise<void>;
    // updateChannel: (id: string, updates: Partial<NotificationChannel>) => void; // Not supported yet
    deleteChannel: (id: string) => Promise<void>;
    fetchChannels: () => Promise<void>;

    updateUser: (data: { password?: string; timezone?: string }) => Promise<void>;

    // Status Pages
    fetchStatusPages: () => Promise<StatusPage[]>;
    toggleStatusPage: (slug: string, publicStatus: boolean, title?: string, groupId?: string) => Promise<void>;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    fetchPublicStatusBySlug: (slug: string) => Promise<any>;

    // API Keys
    fetchAPIKeys: () => Promise<APIKey[]>;
    createAPIKey: (name: string) => Promise<string | null>;
    deleteAPIKey: (id: number) => Promise<void>;
    resetDatabase: () => Promise<boolean>;

    // Settings
    settings: Settings | null;
    fetchSettings: () => Promise<void>;
    updateSettings: (settings: Partial<Settings>) => Promise<void>;

    fetchSystemStats: () => Promise<SystemStats | null>;

    // Setup
    checkSetupStatus: () => Promise<boolean>;
    performSetup: (data: SetupPayload) => Promise<boolean>;
}

export interface SetupPayload {
    username?: string;
    password?: string;
    timezone?: string;
    createDefaults?: boolean;
}

export const useMonitorStore = create<MonitorStore>((set, get) => ({
    groups: [],
    overview: [],
    incidents: [],
    systemEvents: { active: [], history: [] },
    channels: [],
    user: null,
    isAuthChecked: false,
    isSetupComplete: false,
    apiKeys: [],
    settings: null,

    // Sync Actions
    setGroups: (groups: Group[]) => set({ groups }),
    setSystemEvents: (events: { active: SystemIncident[], history: SystemIncident[] }) => set({ systemEvents: events }),

    // Actions
    checkSetupStatus: async () => {
        try {
            const res = await fetch("/api/setup/status");
            if (res.ok) {
                const data = await res.json();
                set({ isSetupComplete: data.isSetup });
                return data.isSetup;
            }
        } catch (e) {
            console.error(e);
        }
        return false;
    },

    performSetup: async (data: SetupPayload) => {
        try {
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), 10000); // 10s timeout

            const res = await fetch("/api/setup", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(data),
                signal: controller.signal
            });
            clearTimeout(timeoutId);

            if (res.ok) {
                set({ isSetupComplete: true });
                return true;
            } else {
                return false;
            }
        } catch (e) {
            console.error("Setup failed or timed out", e);
            return false;
        }
    },
    fetchSystemEvents: async () => {
        try {
            const res = await fetch("/api/events", { credentials: "include" });
            if (res.ok) {
                const data = await res.json();
                set({ systemEvents: data });
            }
        } catch (e) {
            console.error(e);
        }
    },
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
                        isAuthenticated: true,
                        timezone: data.user.timezone
                    },
                    isAuthChecked: true
                });
                // Once auth is confirmed, fetch overview for sidebar/dashboard
                get().fetchOverview();
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
                get().fetchOverview();
                return { success: true };
            }

            if (res.status === 401) {
                return { success: false, error: "Invalid username or password" };
            }
            if (res.status >= 500) {
                return { success: false, error: "Server error. Please try again later." };
            }
            return { success: false, error: "Login failed" };

        } catch (e) {
            console.error(e);
            return { success: false, error: "Network connection error" };
        }
    },

    logout: async () => {
        try {
            await fetch('/api/auth/logout', { method: 'POST', credentials: 'include' });
        } catch (e) {
            console.error(e);
        }
        set({ user: null, groups: [] }); // Clear data on logout
    },

    fetchMonitors: async (groupId?: string) => {
        try {
            const url = groupId ? `/api/uptime?group_id=${groupId}` : '/api/uptime';
            const res = await fetch(url, { credentials: 'include' });
            if (res.ok) {
                const data = await res.json();
                // Backend now returns { groups: [...] }
                set({ groups: data.groups || [] });
            }
        } catch (e) {
            console.error("Failed to fetch monitors", e);
        }
    },

    fetchOverview: async () => {
        try {
            const res = await fetch('/api/overview', { credentials: 'include' });
            if (res.ok) {
                const data = await res.json();
                set({ overview: data.groups || [] });
            }
        } catch (e) {
            console.error("Failed to fetch overview", e);
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

            // Refresh sidebar overview immediately
            get().fetchOverview();

            toast({
                title: "Group Created",
                description: `Group "${name}" created successfully.`,
            });
            return group.id;
        } catch (error: unknown) {
            toast({
                title: "Failed to create group",
                description: (error as Error).message,
                variant: "destructive",
            });
            return undefined;
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
            });
        } catch (error: unknown) {
            toast({
                title: "Failed to update group",
                description: (error as Error).message,
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
                get().fetchOverview();
                toast({ title: "Group Deleted", description: "Group deleted successfully." });
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to delete group.", variant: "destructive" });
        }
    },

    addMonitor: async (name, url, groupName, interval = 60) => {
        const groups = get().groups;
        let groupId = groups.find(g => g.name === groupName)?.id;

        if (!groupId) {
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
                body: JSON.stringify({ name, url, groupId, interval }),
                credentials: 'include'
            });
            if (res.ok) {
                if (groupId) {
                    get().fetchMonitors(groupId);
                }
                toast({ title: "Monitor Created", description: `Monitor "${name}" created successfully.` });
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to create monitor.", variant: "destructive" });
        }
    },

    updateMonitor: async (id, updates) => {
        // Find group ID for this monitor to refresh ONLY that group
        const groups = get().groups;
        let groupId: string | undefined;

        for (const g of groups) {
            if (g.monitors.some(m => m.id === id)) {
                groupId = g.id;
                break;
            }
        }

        try {
            const res = await fetch(`/api/monitors/${id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(updates),
                credentials: 'include'
            });
            if (res.ok) {
                if (groupId) {
                    get().fetchMonitors(groupId);
                }
                // Also refresh overview as status might change
                get().fetchOverview();
                toast({ title: "Monitor Updated", description: "Monitor details updated." });
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to update monitor.", variant: "destructive" });
        }
    },

    deleteMonitor: async (id) => {
        const groups = get().groups;
        let groupId: string | undefined;
        for (const g of groups) {
            if (g.monitors.some(m => m.id === id)) {
                groupId = g.id;
                break;
            }
        }

        try {
            const res = await fetch(`/api/monitors/${id}`, {
                method: 'DELETE',
                credentials: 'include'
            });
            if (res.ok) {
                if (groupId) {
                    get().fetchMonitors(groupId);
                }
                get().fetchOverview();
                toast({ title: "Monitor Deleted", description: "Monitor deleted successfully." });
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to delete monitor.", variant: "destructive" });
        }
    },

    addIncident: async (incident) => {
        try {
            const res = await fetch('/api/incidents', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(incident),
                credentials: 'include'
            });
            if (res.ok) {
                const newIncident = await res.json();
                set((state) => ({ incidents: [newIncident, ...state.incidents] }));
                toast({ title: "Incident Created", description: "Incident has been reported." });
                get().fetchIncidents();
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to create incident.", variant: "destructive" });
        }
    },

    addMaintenance: async (maintenance) => {
        try {
            const res = await fetch('/api/maintenance', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(maintenance),
                credentials: 'include'
            });
            if (res.ok) {
                const newMaintenance = await res.json();
                set((state) => ({ incidents: [newMaintenance, ...state.incidents] }));
                toast({ title: "Maintenance Scheduled", description: "Maintenance window created." });
                get().fetchIncidents();
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to schedule maintenance.", variant: "destructive" });
        }
    },

    fetchIncidents: async () => {
        try {
            const [resIncidents, resMaintenance] = await Promise.all([
                fetch('/api/incidents', { credentials: 'include' }),
                fetch('/api/maintenance', { credentials: 'include' })
            ]);

            let allEvents: Incident[] = [];

            if (resIncidents.ok) {
                const incidents = await resIncidents.json();
                allEvents = [...allEvents, ...(incidents || [])];
            }
            if (resMaintenance.ok) {
                const maintenance = await resMaintenance.json();
                allEvents = [...allEvents, ...(maintenance || [])];
            }

            // Sort desc by start time or created at? Usually StartTime for display
            allEvents.sort((a, b) => new Date(b.startTime).getTime() - new Date(a.startTime).getTime());

            set({ incidents: allEvents });
        } catch (e) {
            console.error("Failed to fetch incidents", e);
        }
    },

    resolveIncident: (id) => set((state) => ({
        incidents: state.incidents.map(inc => inc.id === id ? { ...inc, status: 'resolved' as const } : inc)
    })),

    fetchChannels: async () => {
        try {
            const res = await fetch("/api/notifications/channels", { credentials: "include" });
            if (res.ok) {
                const data = await res.json();
                set({ channels: data.channels || [] });
            }
        } catch (e) {
            console.error("Failed to fetch channels", e);
        }
    },

    addChannel: async (channel) => {
        try {
            // Ensure enabled is true by default
            const payload = { ...channel, enabled: true };
            const res = await fetch("/api/notifications/channels", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(payload),
                credentials: "include"
            });
            if (res.ok) {
                toast({ title: "Channel Added", description: "Notification channel added successfully." });
                get().fetchChannels();
            } else {
                toast({ title: "Error", description: "Failed to add channel.", variant: "destructive" });
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to add channel.", variant: "destructive" });
        }
    },

    deleteChannel: async (id) => {
        try {
            const res = await fetch(`/api/notifications/channels/${id}`, {
                method: "DELETE",
                credentials: "include"
            });
            if (res.ok) {
                toast({ title: "Channel Deleted", description: "Channel deleted successfully." });
                get().fetchChannels();
            }
        } catch (e) {
            console.error(e);
            toast({ title: "Error", description: "Failed to delete channel.", variant: "destructive" });
        }
    },

    fetchAPIKeys: async () => {
        try {
            const res = await fetch("/api/api-keys", { credentials: "include" });
            if (res.ok) {
                const data = await res.json();
                set({ apiKeys: data.keys || [] });
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

    updateSettings: async (newSettings: Partial<Settings>) => {
        try {
            await fetch('/api/settings', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(newSettings),
                credentials: 'include'
            });
            set((state) => ({
                settings: {
                    ...(state.settings || { latency_threshold: "1000", data_retention_days: "30" }),
                    ...newSettings
                }
            }));
        } catch (error) {
            console.error('Failed to update settings:', error);
        }
    },

    fetchSystemStats: async () => {
        try {
            const res = await fetch("/api/stats", { credentials: "include" });
            if (res.ok) {
                return await res.json();
            }
        } catch (error) {
            console.error("Failed to fetch system stats:", error);
        }
        return null;
    },

}));
