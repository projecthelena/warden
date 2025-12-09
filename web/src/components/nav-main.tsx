"use client"

import { ChevronRight, Folder, MoreHorizontal, Pencil, Server, Trash2, type LucideIcon } from "lucide-react"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  useSidebar,
} from "@/components/ui/sidebar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Group, useMonitorStore } from "@/lib/store"
import { EditGroupSheet } from "./EditGroupSheet"
import { useState } from "react"
import { useLocation, Link } from "react-router-dom"

export function NavMain({
  items,
  groups,
}: {
  items: {
    title: string
    url: string
    icon: LucideIcon
    isActive?: boolean
    items?: {
      title: string
      url: string
    }[]
  }[]
  groups: Group[]
}) {
  const { isMobile, state } = useSidebar()
  const { deleteGroup } = useMonitorStore()
  const [editingGroup, setEditingGroup] = useState<Group | null>(null)
  const { pathname } = useLocation();

  const handleDelete = (group: Group) => {
    if (confirm(`Are you sure you want to delete group "${group.name}"?`)) {
      deleteGroup(group.id)
    }
  }

  return (
    <>
      <SidebarGroup>
        <SidebarGroupLabel>Platform</SidebarGroupLabel>
        <SidebarMenu>
          {items.map((item) => {
            const isMainActive = item.isActive ?? (item.url === pathname);
            const isSubActive = item.items?.some(sub => pathname === sub.url || pathname.startsWith(sub.url + "/"));
            // Open if sub-item is active OR if the main item itself is active (and has items)
            const isOpen = isSubActive || (isMainActive && !!item.items?.length);

            // Smart highlighting logic:
            // Collapsed: Highlight if self OR child matches (shows group is active)
            // Expanded: Highlight ONLY if self matches AND has no children (prevents double highlight with child)
            const isButtonActive = state === "collapsed"
              ? (isMainActive || isSubActive)
              : (isMainActive && !item.items?.length);

            return (
              <Collapsible key={item.title} asChild defaultOpen={isOpen} className="group/collapsible">
                <SidebarMenuItem>
                  <SidebarMenuButton asChild tooltip={item.title} isActive={isButtonActive}>
                    <Link to={item.url}>
                      <item.icon />
                      <span>{item.title}</span>
                    </Link>
                  </SidebarMenuButton>
                  {item.items?.length ? (
                    <>
                      <CollapsibleTrigger asChild>
                        <SidebarMenuAction className="data-[state=open]:rotate-90">
                          <ChevronRight />
                          <span className="sr-only">Toggle</span>
                        </SidebarMenuAction>
                      </CollapsibleTrigger>
                      <CollapsibleContent>
                        <SidebarMenuSub>
                          {item.items?.map((subItem) => (
                            <SidebarMenuSubItem key={subItem.title}>
                              <SidebarMenuSubButton asChild isActive={pathname === subItem.url}>
                                <Link to={subItem.url}>
                                  <span>{subItem.title}</span>
                                </Link>
                              </SidebarMenuSubButton>
                            </SidebarMenuSubItem>
                          ))}
                        </SidebarMenuSub>
                      </CollapsibleContent>
                    </>
                  ) : null}
                </SidebarMenuItem>
              </Collapsible>
            )
          })}

          <Collapsible key="Groups" asChild defaultOpen={pathname.startsWith("/groups") || pathname === "/dashboard"} className="group/collapsible">
            <SidebarMenuItem>
              <SidebarMenuButton tooltip="Groups">
                <Folder />
                <span>Groups</span>
              </SidebarMenuButton>
              <CollapsibleTrigger asChild>
                <SidebarMenuAction className="data-[state=open]:rotate-90">
                  <ChevronRight />
                  <span className="sr-only">Toggle</span>
                </SidebarMenuAction>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <SidebarMenuSub>
                  {groups.map((group) => {
                    const groupUrl = `/groups/${group.id}`;
                    // Dashboard (root) often maps to default group, handling that edge case might be nice, but explicit ID check is safer
                    const isActive = pathname === groupUrl;

                    return (
                      <SidebarMenuSubItem key={group.id}>
                        <SidebarMenuSubButton asChild isActive={isActive}>
                          <Link to={groupUrl}>
                            <span>{group.name}</span>
                          </Link>
                        </SidebarMenuSubButton>
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
                      </SidebarMenuSubItem>
                    )
                  })}
                </SidebarMenuSub>
              </CollapsibleContent>
            </SidebarMenuItem>
          </Collapsible>
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
