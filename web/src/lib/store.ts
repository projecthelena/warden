import { create } from 'zustand';

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

export interface Monitor {
    id: string;
    name: string;
    url: string;
    status: 'up' | 'down' | 'degraded';
    latency: number;
    history: ('up' | 'down' | 'degraded')[];
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
    login: (username: string) => Promise<boolean>;
    logout: () => Promise<void>;

    // CRUD
    fetchPublicStatus: () => Promise<void>;
    fetchMonitors: () => Promise<void>;
    addGroup: (name: string) => Promise<void>;
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
}

export interface StatusPage {
    id: number;
    slug: string;
    title: string;
    groupId?: string;
    public: boolean;
    createdAt: string;
}

export const useMonitorStore = create<MonitorStore>((set, get) => ({
    groups: [],
    incidents: [],
    channels: [],
    user: null,
    isAuthChecked: false,

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
                if (data.groups) {
                    set({ groups: data.groups });
                }
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
    addGroup: async (name) => {
        try {
            const res = await fetch('/api/groups', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name }),
                credentials: 'include'
            });
            if (res.ok) {
                get().fetchMonitors();
            }
        } catch (e) {
            console.error(e);
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
            }
        } catch (e) {
            console.error(e);
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
            }
        } catch (e) {
            console.error(e);
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
            }
        } catch (e) {
            console.error(e);
        }
    },

    addIncident: (incident) => set((state) => ({
        incidents: [{ ...incident, id: Math.random().toString(36).substr(2, 9) }, ...state.incidents]
    })),
    addChannel: (channel) => set((state) => ({
        channels: [...state.channels, { ...channel, id: Math.random().toString(36).substr(2, 9) }]
    })),
    updateChannel: (id, updates) => { },
    deleteChannel: (id) => { }
}));
