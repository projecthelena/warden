import {
    Sidebar,
    SidebarContent,
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarHeader,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarRail,
} from "@/components/ui/sidebar";
import { Activity, Layers, Server, Zap } from "lucide-react";
import { Group } from "@/lib/store";
import { useNavigate, useLocation } from "react-router-dom";

interface AppSidebarProps {
    groups: Group[];
}

export function AppSidebar({ groups }: AppSidebarProps) {
    const navigate = useNavigate();
    const location = useLocation();

    const isActive = (path: string) => {
        if (path === '/') return location.pathname === '/';
        return location.pathname.startsWith(path);
    }

    return (
        <Sidebar collapsible="icon" className="border-r border-slate-800 bg-slate-950">
            <SidebarHeader>
                <SidebarMenu>
                    <SidebarMenuItem>
                        <SidebarMenuButton size="lg" className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground">
                            <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-blue-600 text-sidebar-primary-foreground">
                                <Activity className="size-4" />
                            </div>
                            <div className="grid flex-1 text-left text-sm leading-tight">
                                <span className="truncate font-semibold">ClusterUptime</span>
                                <span className="truncate text-xs text-slate-400">OSS Monitor</span>
                            </div>
                        </SidebarMenuButton>
                    </SidebarMenuItem>
                </SidebarMenu>
            </SidebarHeader>
            <SidebarContent>
                <SidebarGroup>
                    <SidebarGroupLabel>Dashboards</SidebarGroupLabel>
                    <SidebarGroupContent>
                        <SidebarMenu>
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    isActive={location.pathname === '/'}
                                    onClick={() => navigate('/')}
                                >
                                    <Layers />
                                    <span>All Groups</span>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    isActive={location.pathname === '/incidents'}
                                    onClick={() => navigate('/incidents')}
                                >
                                    <Zap />
                                    <span>Incidents</span>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                            <SidebarMenuItem>
                                <SidebarMenuButton
                                    isActive={location.pathname === '/status'}
                                    onClick={() => navigate('/status')}
                                >
                                    <Activity />
                                    <span>Status Page</span>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                        </SidebarMenu>
                    </SidebarGroupContent>
                </SidebarGroup>

                <SidebarGroup>
                    <SidebarGroupLabel>Groups</SidebarGroupLabel>
                    <SidebarGroupContent>
                        <SidebarMenu>
                            {groups.map((group) => (
                                <SidebarMenuItem key={group.id}>
                                    <SidebarMenuButton
                                        isActive={location.pathname === `/groups/${group.id}`}
                                        onClick={() => navigate(`/groups/${group.id}`)}
                                    >
                                        <Server />
                                        <span>{group.name}</span>
                                    </SidebarMenuButton>
                                </SidebarMenuItem>
                            ))}
                        </SidebarMenu>
                    </SidebarGroupContent>
                </SidebarGroup>
            </SidebarContent>
            <SidebarRail />
        </Sidebar>
    );
}
