"use client"

import { ChevronRight, Folder, type LucideIcon } from "lucide-react"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  useSidebar,
} from "@/components/ui/sidebar"
import { Group, OverviewGroup } from "@/lib/store"
import { useLocation, Link } from "react-router-dom"

export function NavMain({
  items,
  groups,
  settings,
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
  groups: (Group | OverviewGroup)[]
  settings?: {
    title: string
    url: string
    icon: LucideIcon
    isActive?: boolean
    items?: {
      title: string
      url: string
    }[]
  }[]
}) {
  const { state } = useSidebar()
  const { pathname } = useLocation();

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const renderItems = (items: any[]) => {
    return items.map((item) => {
      const isMainActive = item.isActive ?? (item.url === pathname);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const isSubActive = item.items?.some((sub: any) => pathname === sub.url || pathname.startsWith(sub.url + "/"));
      const isOpen = isSubActive || (isMainActive && !!item.items?.length);

      if (item.items?.length) {
        // Collapsible Parent (Events, Settings)
        const isButtonActive = state === "collapsed" ? (isMainActive || isSubActive) : false;

        return (
          <Collapsible key={item.title} asChild defaultOpen={isOpen} className="group/collapsible">
            <SidebarMenuItem>
              <CollapsibleTrigger asChild>
                <SidebarMenuButton tooltip={item.title} isActive={isButtonActive}>
                  <item.icon />
                  <span>{item.title}</span>
                  <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
                </SidebarMenuButton>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <SidebarMenuSub>
                  {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                  {item.items?.map((subItem: any) => (
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
            </SidebarMenuItem>
          </Collapsible>
        )
      } else {
        // Direct Link (Overview, Status Pages)
        return (
          <SidebarMenuItem key={item.title}>
            <SidebarMenuButton asChild tooltip={item.title} isActive={isMainActive}>
              <Link to={item.url}>
                <item.icon />
                <span>{item.title}</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        )
      }
    })
  }

  return (
    <>
      <SidebarGroup>
        <SidebarGroupLabel>Platform</SidebarGroupLabel>
        <SidebarMenu>
          {renderItems(items)}

          <Collapsible key="Groups" asChild defaultOpen={pathname.startsWith("/groups") || pathname === "/dashboard"} className="group/collapsible">
            <SidebarMenuItem>
              <CollapsibleTrigger asChild>
                <SidebarMenuButton tooltip="Groups">
                  <Folder />
                  <span>Groups</span>
                  <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
                </SidebarMenuButton>
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
                      </SidebarMenuSubItem>
                    )
                  })}
                </SidebarMenuSub>
              </CollapsibleContent>
            </SidebarMenuItem>
          </Collapsible>

          {settings && renderItems(settings)}
        </SidebarMenu>
      </SidebarGroup>
    </>
  )
}
