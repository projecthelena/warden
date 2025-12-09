


import * as React from "react"
import {
    BookOpen,
    Bot,
    Command,
    Frame,
    LifeBuoy,
    Map,
    PieChart,
    Send,
    Settings2,
    Terminal,
    Activity,
    Zap,
    Bell,
} from "lucide-react"

import { NavMain } from "@/components/nav-main"
import { NavGroups } from "@/components/nav-groups"
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
import { Group, useMonitorStore } from "@/lib/store"

export function AppSidebar({ groups, ...props }: React.ComponentProps<typeof Sidebar> & { groups: Group[] }) {
    const { user } = useMonitorStore();

    const data = {
        user: {
            name: user?.name || "User",
            email: user?.email || "user@example.com",
            avatar: user?.avatar || "/avatars/shadcn.jpg",
        },
        navMain: [
            {
                title: "Dashboards",
                url: "/dashboard",
                icon: Terminal,
                isActive: true,
                items: [
                    {
                        title: "Overview",
                        url: "/dashboard",
                    },
                ],
            },
            {
                title: "Incidents",
                url: "/incidents",
                icon: Zap,
                items: [
                    {
                        title: "Active Incidents",
                        url: "/incidents",
                    },
                    {
                        title: "Maintenance",
                        url: "/incidents", // Could be filtered view in future
                    },
                ],
            },

            {
                title: "Status Pages",
                url: "/status-pages",
                icon: Activity,
                items: [
                    {
                        title: "Manage Pages",
                        url: "/status-pages",
                    },
                ],
            },
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
                ],
            },
        ],
        navSecondary: [
            {
                title: "Support",
                url: "#",
                icon: LifeBuoy,
            },
            {
                title: "Feedback",
                url: "#",
                icon: Send,
            },
        ],
    }

    return (
        <Sidebar variant="inset" {...props}>
            <SidebarHeader>
                <SidebarMenu>
                    <SidebarMenuItem>
                        <SidebarMenuButton size="lg" asChild>
                            <a href="#">
                                <div className="bg-sidebar-primary text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg">
                                    <Command className="size-4" />
                                </div>
                                <div className="grid flex-1 text-left text-sm leading-tight">
                                    <span className="truncate font-medium">ClusterUptime</span>
                                    <span className="truncate text-xs">Enterprise</span>
                                </div>
                            </a>
                        </SidebarMenuButton>
                    </SidebarMenuItem>
                </SidebarMenu>
            </SidebarHeader>
            <SidebarContent>
                <NavMain items={data.navMain} />
                <NavGroups groups={groups} />
                <NavSecondary items={data.navSecondary} className="mt-auto" />
            </SidebarContent>
            <SidebarFooter>
                <NavUser user={data.user} />
            </SidebarFooter>
        </Sidebar>
    )
}
