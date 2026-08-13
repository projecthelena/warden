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
import { Group, OverviewGroup, findMonitorWithGroup } from "@/lib/store"
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
  const { pathname, search } = useLocation();
  const fullPath = pathname + search;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const renderItems = (items: any[]) => {
    return items.map((item) => {
      const isMainActive = item.isActive ?? (item.url === pathname);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const isSubActive = item.items?.some((sub: any) => {
        if (sub.url.includes("?")) {
          return fullPath === sub.url;
        }
        // For items without query params, match if pathname equals the URL and there's no tab param
        if (pathname === sub.url) {
          return !search.includes("tab=");
        }
        return pathname.startsWith(sub.url + "/");
      });
      const isOpen = isSubActive || (isMainActive && !!item.items?.length);

      if (item.items?.length === 1) {
        // Single sub-item: render as direct link (e.g. viewer sees Settings → General only)
        const single = item.items[0];
        const isActive = single.url.includes("?")
          ? fullPath === single.url
          : pathname === single.url && !search.includes("tab=");
        return (
          <SidebarMenuItem key={item.title}>
            <SidebarMenuButton asChild tooltip={item.title} isActive={isActive}>
              <Link to={single.url}>
                <item.icon />
                <span>{item.title}</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        )
      }

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
                  {item.items?.map((subItem: any) => {
                    const isActive = subItem.url.includes("?")
                      ? fullPath === subItem.url
                      : pathname === subItem.url && !search.includes("tab=");
                    return (
                      <SidebarMenuSubItem key={subItem.title}>
                        <SidebarMenuSubButton asChild isActive={isActive}>
                          <Link to={subItem.url}>
                            <span>{subItem.title}</span>
                          </Link>
                        </SidebarMenuSubButton>
                      </SidebarMenuSubItem>
                    );
                  })}
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

          {(() => {
            // When viewing /monitors/:id, expand the Groups accordion and highlight the
            // sub-item that owns that monitor — gives the user a clear "you are here"
            // anchor without polluting the route table with a /groups/:gid/monitors/:mid
            // shape.
            const onMonitorRoute = pathname.startsWith("/monitors/");
            const monitorId = onMonitorRoute ? pathname.split("/")[2] : "";
            const owningGroup = onMonitorRoute
              ? findMonitorWithGroup(groups as Group[], monitorId)?.group
              : null;
            const collapsibleOpen =
              pathname.startsWith("/groups") || pathname === "/dashboard" || onMonitorRoute;

            return (
              <Collapsible key="Groups" asChild defaultOpen={collapsibleOpen} className="group/collapsible">
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
                        const isActive = pathname === groupUrl || owningGroup?.id === group.id;
                        return (
                          <SidebarMenuSubItem key={group.id}>
                            <SidebarMenuSubButton asChild isActive={isActive}>
                              <Link to={groupUrl}>
                                <span>{group.name}</span>
                              </Link>
                            </SidebarMenuSubButton>
                          </SidebarMenuSubItem>
                        );
                      })}
                    </SidebarMenuSub>
                  </CollapsibleContent>
                </SidebarMenuItem>
              </Collapsible>
            );
          })()}

          {settings && renderItems(settings)}
        </SidebarMenu>
      </SidebarGroup>
    </>
  )
}
