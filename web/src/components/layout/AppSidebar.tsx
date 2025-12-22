


import * as React from "react"
import { Link, useLocation } from "react-router-dom"
import {
    Activity,
    LayoutDashboard,
    LifeBuoy,
    Settings2,
    Siren,
} from "lucide-react"

import { NavMain } from "@/components/nav-main"
import { NavSecondary } from "@/components/nav-secondary"
import { NavUser } from "@/components/nav-user"
import {
    Sidebar,
    SidebarContent,
    SidebarFooter,
    SidebarHeader,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
} from "@/components/ui/sidebar"
import { Group, OverviewGroup, useMonitorStore } from "@/lib/store"

export function AppSidebar({ groups, ...props }: React.ComponentProps<typeof Sidebar> & { groups: (Group | OverviewGroup)[] }) {
    const { user } = useMonitorStore();
    const { pathname } = useLocation();

    const data = {
        user: {
            name: user?.name || "User",
            email: user?.email || "user@example.com",
            avatar: user?.avatar || "/avatars/shadcn.jpg",
        },
        navMain: [
            {
                title: "Overview",
                url: "/dashboard",
                icon: LayoutDashboard,
                isActive: pathname === "/dashboard",
            },
            {
                title: "Status Pages",
                url: "/status-pages",
                icon: Activity,
                isActive: pathname === "/status-pages",
            },
            {
                title: "Events",
                url: "#",
                icon: Siren,
                isActive: pathname.startsWith("/incidents") || pathname.startsWith("/maintenance"),
                items: [
                    {
                        title: "Incidents",
                        url: "/incidents",
                        isActive: pathname === "/incidents",
                    },
                    {
                        title: "Maintenance",
                        url: "/maintenance",
                        isActive: pathname === "/maintenance",
                    },
                ],
            },
        ],
        navSettings: [
            {
                title: "Settings",
                url: "/settings",
                icon: Settings2,
                items: [
                    {
                        title: "General",
                        url: "/settings",
                    },
                    {
                        title: "Notifications",
                        url: "/notifications",
                    },
                    {
                        title: "API Keys",
                        url: "/settings/api-keys",
                    },
                ],
            },
        ],
        navSecondary: [
            {
                title: "Support",
                url: "https://github.com/ClusterUptime/clusteruptime/issues/new",
                icon: LifeBuoy,
            },
        ],
    }

    return (
        <Sidebar variant="inset" {...props}>
            <SidebarHeader>
                <SidebarMenu>
                    <SidebarMenuItem>
                        <SidebarMenuButton size="lg" asChild>
                            <Link to="/dashboard">
                                <div className="flex aspect-square size-8 items-center justify-center rounded-lg">
                                    <Activity className="size-6 text-cyan-400" />
                                </div>
                                <div className="grid flex-1 text-left text-sm leading-tight">
                                    <span className="truncate font-semibold">ClusterUptime</span>
                                    <span className="truncate text-xs">OSS Monitor</span>
                                </div>
                            </Link>
                        </SidebarMenuButton>
                    </SidebarMenuItem>
                </SidebarMenu>
            </SidebarHeader>
            <SidebarContent>
                <NavMain items={data.navMain} groups={groups} settings={data.navSettings} />
            </SidebarContent>
            <SidebarFooter>
                <NavSecondary items={data.navSecondary} />
                <NavUser user={data.user} />
            </SidebarFooter>
        </Sidebar>
    )
}
