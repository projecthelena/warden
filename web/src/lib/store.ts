import { useState, useEffect } from 'react';

export interface Monitor {
    id: string;
    name: string;
    url: string;
    status: 'up' | 'down' | 'degraded';
    latency: number;
    history: ('up' | 'down' | 'degraded')[]; // Last 20 checks
    lastCheck: string;
}

export interface Project {
    id: string;
    name: string;
    monitors: Monitor[];
}

const generateHistory = (): Monitor['history'] => {
    return Array.from({ length: 20 }, () => {
        const r = Math.random();
        if (r > 0.98) return 'down';
        if (r > 0.95) return 'degraded';
        return 'up';
    });
};

const INITIAL_DATA: Project[] = [
    {
        id: "p1",
        name: "Platform Core",
        monitors: [
            { id: "m1", name: "Auth Service", url: "https://auth.api.internal", status: "up", latency: 45, history: generateHistory(), lastCheck: "Just now" },
            { id: "m2", name: "User Database", url: "db.prod.internal", status: "up", latency: 12, history: generateHistory(), lastCheck: "Just now" },
            { id: "m3", name: "Payment Gateway", url: "stripe.api", status: "degraded", latency: 850, history: generateHistory(), lastCheck: "1m ago" },
        ]
    },
    {
        id: "p2",
        name: "Marketing Site",
        monitors: [
            { id: "m4", name: "Landing Page", url: "https://clustercost.com", status: "up", latency: 120, history: generateHistory(), lastCheck: "Just now" },
            { id: "m5", name: "Blog", url: "https://blog.clustercost.com", status: "up", latency: 150, history: generateHistory(), lastCheck: "Just now" },
        ]
    },
    {
        id: "p3",
        name: "Internal Tools",
        monitors: [
            { id: "m6", name: "CI/CD Pipeline", url: "jenkins.internal", status: "down", latency: 0, history: generateHistory(), lastCheck: "5m ago" }
        ]
    }
];

export const useMonitorStore = () => {
    const [projects, setProjects] = useState<Project[]>(INITIAL_DATA);

    useEffect(() => {
        // Simulate live updates
        const interval = setInterval(() => {
            setProjects(prev => prev.map(p => ({
                ...p,
                monitors: p.monitors.map(m => {
                    // Randomly fluctuate latency and occasionally status
                    const newLatency = Math.max(10, m.latency + (Math.random() * 40 - 20));
                    let newStatus = m.status;
                    if (Math.random() > 0.99) newStatus = 'down';
                    else if (Math.random() > 0.99) newStatus = 'degraded';
                    else if (Math.random() > 0.95) newStatus = 'up';

                    // Shift history
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

    return { projects };
};
