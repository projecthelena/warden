"use client"

import {
    MoreHorizontal,
    Trash2,
    Server,
    Pencil,
} from "lucide-react"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    SidebarGroup,
    SidebarGroupLabel,
    SidebarMenu,
    SidebarMenuAction,
    SidebarMenuButton,
    SidebarMenuItem,
    useSidebar,
} from "@/components/ui/sidebar"
import { Group, useMonitorStore } from "@/lib/store";
import { EditGroupSheet } from "./EditGroupSheet";
import { useState } from "react";

export function NavGroups({
    groups,
}: {
    groups: Group[]
}) {
    const { isMobile } = useSidebar();
    const { deleteGroup } = useMonitorStore();
    const [editingGroup, setEditingGroup] = useState<Group | null>(null);

    const handleDelete = (group: Group) => {
        if (confirm(`Are you sure you want to delete group "${group.name}"?`)) {
            deleteGroup(group.id);
        }
    };

    return (
        <>
            <SidebarGroup className="group-data-[collapsible=icon]:hidden">
                <SidebarGroupLabel>Groups</SidebarGroupLabel>
                <SidebarMenu>
                    {groups.map((group) => (
                        <SidebarMenuItem key={group.id}>
                            <SidebarMenuButton asChild>
                                <a href={`/groups/${group.id}`}>
                                    <Server />
                                    <span>{group.name}</span>
                                </a>
                            </SidebarMenuButton>
                            <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                    <SidebarMenuAction showOnHover>
                                        <MoreHorizontal />
                                        <span className="sr-only">More</span>
                                    </SidebarMenuAction>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent
                                    className="w-48 bg-slate-950 border-slate-800 text-slate-100"
                                    side={isMobile ? "bottom" : "right"}
                                    align={isMobile ? "end" : "start"}
                                >
                                    <DropdownMenuItem onClick={() => setEditingGroup(group)} className="focus:bg-slate-900 focus:text-slate-100 cursor-pointer">
                                        <Pencil className="text-muted-foreground mr-2 h-4 w-4" />
                                        <span>Edit Group</span>
                                    </DropdownMenuItem>
                                    {group.id !== 'g-default' && (
                                        <>
                                            <DropdownMenuSeparator className="bg-slate-800" />
                                            <DropdownMenuItem onClick={() => handleDelete(group)} className="text-red-400 focus:bg-red-950/30 focus:text-red-300 cursor-pointer">
                                                <Trash2 className="text-muted-foreground mr-2 h-4 w-4" />
                                                <span>Delete Group</span>
                                            </DropdownMenuItem>
                                        </>
                                    )}
                                </DropdownMenuContent>
                            </DropdownMenu>
                        </SidebarMenuItem>
                    ))}
                    {/* <SidebarMenuItem>
          <SidebarMenuButton className="text-sidebar-foreground/70">
            <MoreHorizontal className="text-sidebar-foreground/70" />
            <span>More</span>
          </SidebarMenuButton>
        </SidebarMenuItem> */}
                </SidebarMenu>
            </SidebarGroup>

            <EditGroupSheet
                group={editingGroup}
                open={!!editingGroup}
                onOpenChange={(open) => !open && setEditingGroup(null)}
                onSave={(id, name) => {
                    const { updateGroup } = useMonitorStore.getState();
                    updateGroup(id, name);
                }}
            />
        </>
    )
}
