import { useState, useEffect } from 'react';

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

const generateHistory = (): Monitor['history'] => {
    return Array.from({ length: 20 }, () => {
        const r = Math.random();
        if (r > 0.98) return 'down';
        if (r > 0.95) return 'degraded';
        return 'up';
    });
};

// Helper to generate some mock events
const generateEvents = (): MonitorEvent[] => {
    return [
        { id: '1', type: 'up', timestamp: new Date(Date.now() - 1000 * 60 * 5).toISOString(), message: 'Monitor is UP (200 OK)' },
        { id: '2', type: 'degraded', timestamp: new Date(Date.now() - 1000 * 60 * 60).toISOString(), message: 'High latency detected (850ms)' },
        { id: '3', type: 'up', timestamp: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString(), message: 'Monitor recovered' },
    ]
}

// Initialize with a default group
const INITIAL_GROUPS: Group[] = [
    {
        id: "default",
        name: "Default",
        monitors: []
    },
    {
        id: "g1",
        name: "Platform Core",
        monitors: [
            { id: "m1", name: "Auth Service", url: "https://auth.api.internal", status: "up", latency: 45, history: generateHistory(), lastCheck: "Just now", events: generateEvents() },
            { id: "m2", name: "User Database", url: "db.prod.internal", status: "up", latency: 12, history: generateHistory(), lastCheck: "Just now", events: [] },
        ]
    }
];

const INITIAL_INCIDENTS: Incident[] = [
    {
        id: "i1",
        title: "Payment Gateway Latency",
        description: "We are observing high latency check.",
        status: "investigating",
        type: "incident",
        severity: "major",
        startTime: new Date().toISOString(),
        affectedGroups: ["Platform Core"]
    }
]

const INITIAL_CHANNELS: NotificationChannel[] = [
    {
        id: "c1",
        type: "slack",
        name: "DevOps Alerts",
        config: { webhookUrl: "https://hooks.slack.com/services/..." },
        enabled: true
    }
];

export const useMonitorStore = () => {
    const [groups, setGroups] = useState<Group[]>(INITIAL_GROUPS);
    const [incidents, setIncidents] = useState<Incident[]>(INITIAL_INCIDENTS);
    const [channels, setChannels] = useState<NotificationChannel[]>(INITIAL_CHANNELS);

    const addGroup = (name: string) => {
        setGroups(prev => {
            if (prev.some(g => g.name.toLowerCase() === name.toLowerCase())) return prev;
            return [...prev, {
                id: Math.random().toString(36).substr(2, 9),
                name,
                monitors: []
            }];
        });
    };

    const addMonitor = (name: string, url: string, groupName: string = "Default") => {
        setGroups(prev => {
            const currentGroups = [...prev];
            const targetGroup = groupName.trim() || "Default";
            const existingGroupIndex = currentGroups.findIndex(g => g.name.toLowerCase() === targetGroup.toLowerCase());

            const newMonitor: Monitor = {
                id: Math.random().toString(36).substr(2, 9),
                name,
                url,
                status: 'up',
                latency: Math.floor(Math.random() * 50) + 10,
                history: Array(20).fill('up'),
                lastCheck: "Just now",
                events: []
            };

            if (existingGroupIndex >= 0) {
                // Add to existing
                const group = { ...currentGroups[existingGroupIndex] };
                group.monitors = [...group.monitors, newMonitor];
                currentGroups[existingGroupIndex] = group;
            } else {
                // Create new group
                currentGroups.push({
                    id: Math.random().toString(36).substr(2, 9),
                    name: targetGroup,
                    monitors: [newMonitor]
                });
            }
            return currentGroups;
        });
    };

    const updateMonitor = (monitorId: string, updates: Partial<Monitor>) => {
        setGroups(prev => prev.map(g => ({
            ...g,
            monitors: g.monitors.map(m => m.id === monitorId ? { ...m, ...updates } : m)
        })));
    };

    const deleteMonitor = (monitorId: string) => {
        setGroups(prev => prev.map(g => ({
            ...g,
            monitors: g.monitors.filter(m => m.id !== monitorId)
        })));
    };

    const addIncident = (incident: Omit<Incident, 'id'>) => {
        const newIncident = { ...incident, id: Math.random().toString(36).substr(2, 9) };
        setIncidents(prev => [newIncident, ...prev]);
    };

    const addChannel = (channel: Omit<NotificationChannel, 'id'>) => {
        const newChannel = { ...channel, id: Math.random().toString(36).substr(2, 9) };
        setChannels(prev => [...prev, newChannel]);
    };

    const updateChannel = (id: string, updates: Partial<NotificationChannel>) => {
        setChannels(prev => prev.map(c => c.id === id ? { ...c, ...updates } : c));
    };

    const deleteChannel = (id: string) => {
        setChannels(prev => prev.filter(c => c.id !== id));
    };

    useEffect(() => {
        // Simulate live updates
        const interval = setInterval(() => {
            setGroups(prev => prev.map(g => ({
                ...g,
                monitors: g.monitors.map(m => {
                    const newLatency = Math.max(10, m.latency + (Math.random() * 40 - 20));
                    let newStatus = m.status;
                    if (Math.random() > 0.99) newStatus = 'down';
                    else if (Math.random() > 0.99) newStatus = 'degraded';
                    else if (Math.random() > 0.95) newStatus = 'up';

                    const newHistory = [...m.history.slice(1), newStatus];

                    return {
                        ...m,
                        latency: Math.floor(newLatency),
                        status: newStatus,
                        history: newHistory
                    };
                })
            })));
        }, 2000);
        return () => clearInterval(interval);
    }, []);

    return { groups, incidents, channels, addGroup, addMonitor, updateMonitor, deleteMonitor, addIncident, addChannel, updateChannel, deleteChannel };
};
