


import * as React from "react"
import { Link, useLocation } from "react-router-dom"
import {
    Activity,
    CalendarClock,
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
            name: user?.name || user?.username || "User",
            email: user?.email || "",
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
                title: "Incidents",
                url: "/incidents",
                icon: Siren,
                isActive: pathname.startsWith("/incidents"),
            },
            {
                title: "Maintenance",
                url: "/maintenance",
                icon: CalendarClock,
                isActive: pathname.startsWith("/maintenance"),
            },
        ],
        navSettings: [
            {
                title: "Settings",
                url: "/settings",
                icon: Settings2,
                isActive: pathname.startsWith("/settings"),
            },
        ],
        navSecondary: [
            {
                title: "Support",
                url: "https://github.com/projecthelena/warden/issues/new",
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
                                <div className="grid flex-1 text-left text-sm leading-tight">
                                    <span className="truncate font-bold tracking-tight">
                                        Project <span className="font-normal text-muted-foreground">Helena</span>
                                    </span>
                                    <span className="truncate text-xs font-medium text-cyan-500 tracking-widest">WARDEN</span>
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
