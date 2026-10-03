import { useState, useEffect } from "react";
import React from "react";
import { Routes, Route, useParams, useLocation, useNavigate, Navigate } from "react-router-dom";
import { AppSidebar } from "./components/layout/AppSidebar";
import { SidebarProvider, SidebarInset, SidebarTrigger } from "./components/ui/sidebar";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "./components/ui/separator";

import { useMonitorStore, Group, OverviewGroup, findMonitorWithGroup } from "./lib/store";
import { Card } from "./components/ui/card";
import { ChevronRight } from "lucide-react";
import { CreateMonitorSheet } from "./components/CreateMonitorSheet";
import { CreateGroupSheet } from "./components/CreateGroupSheet";
import { CreateMaintenanceSheet } from "./components/incidents/CreateMaintenanceSheet";
import { MaintenanceView } from "./components/incidents/MaintenanceView";
import { IncidentsView } from "./components/incidents/IncidentsView";
import { StatusPage } from "./components/status-page/StatusPage";
import { LoginPage } from "./components/auth/LoginPage";
import { SettingsView } from "./components/settings/SettingsView";
import { StatusPagesView } from "./components/status-pages/StatusPagesView";
import { MonitorPage } from "./components/MonitorPage";
import { GroupMonitorsPage } from "./components/GroupMonitorsPage";

// Legacy /digest/:date links from older Slack messages redirect into the canonical
// /incidents?date=:date view, so we don't have two parallel surfaces.
const DigestRedirect = () => {
    const { date } = useParams<{ date: string }>();
    return <Navigate to={`/incidents?date=${encodeURIComponent(date || "")}`} replace />;
};
import { Toaster } from "@/components/ui/toaster";
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"



import { useRole } from "@/hooks/useRole";

// New Lightweight Group Card for Overview
// New Lightweight Group Card for Overview (Status Page Style)
function GroupOverviewCard({ group }: { group: OverviewGroup }) {
  const navigate = useNavigate();

  const statusColor =
    group.status === 'up' ? 'bg-green-500' :
      group.status === 'degraded' ? 'bg-yellow-500' :
        group.status === 'maintenance' ? 'bg-blue-500' : 'bg-red-500';

  const statusText =
    group.status === 'up' ? 'Operational' :
      group.status === 'degraded' ? 'Degraded' :
        group.status === 'maintenance' ? 'Maintenance' : 'Unavailable';

  const statusTextColor =
    group.status === 'up' ? 'text-green-500' :
      group.status === 'degraded' ? 'text-yellow-500' :
        group.status === 'maintenance' ? 'text-blue-500' : 'text-red-500';

  return (
    <Card
      onClick={() => navigate(`/groups/${group.id}`)}
      className="group relative flex flex-row items-center justify-between p-4 rounded-xl border-border/50 bg-card/50 hover:bg-accent/50 transition-all duration-300 cursor-pointer overflow-hidden gap-4 shadow-none"
    >
      {/* Hover Glow & Left Border */}
      <div className={`absolute left-0 top-0 bottom-0 w-1 ${statusColor} opacity-0 group-hover:opacity-100 transition-opacity duration-300`} />

      <div className="flex items-center gap-3 pl-2">
        <div className="font-medium text-foreground group-hover:text-foreground transition-colors">
          {group.name}
        </div>
      </div>

      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2.5">
          <div className={`text-sm font-medium ${statusTextColor} transition-colors`}>
            {statusText}
          </div>
          <div className="relative flex items-center justify-center">
            {group.status !== 'up' && group.status !== 'maintenance' && (
              <span className={`absolute inline-flex h-full w-full rounded-full ${statusColor} opacity-75 animate-ping`} />
            )}
            <span className={`relative inline-flex rounded-full h-2.5 w-2.5 ${statusColor}`} />
          </div>
        </div>

        <div className="pl-2 border-l border-border/50">
          <ChevronRight className="w-5 h-5 text-muted-foreground group-hover:text-foreground group-hover:translate-x-1 transition-all duration-300" />
        </div>
      </div>
    </Card>
  )
}

function Dashboard() {
  const overview = useOverviewQuery();
  const groups = overview.data || [];
  const downGroups = groups.filter(g => g.status === 'down').length;
  const degradedGroups = groups.filter(g => g.status === 'degraded').length;
  const isHealthy = downGroups === 0 && degradedGroups === 0;

  return (
    <div className="space-y-6">
      <div><h2 className="text-xl font-semibold tracking-tight text-foreground">{isHealthy ? "All Systems Operational" : "System Issues Detected"}</h2><p className={`text-sm ${isHealthy ? 'text-muted-foreground' : 'text-red-400'}`}>{isHealthy ? `Monitoring ${groups.length} check groups. Everything looks good.` : `${downGroups} groups down, ${degradedGroups} degraded.`}</p></div>
      {overview.isLoading ? <div className="space-y-3">{Array.from({ length: 4 }, (_, index) => <div key={index} className="h-16 animate-pulse rounded-xl bg-muted" />)}</div> : groups.length === 0 ? <div className="text-center text-muted-foreground py-10 border border-border rounded-xl bg-card">No groups found. Create one to get started.</div> : <div className="grid gap-3">{groups.map(group => <GroupOverviewCard key={group.id} group={group} />)}</div>}
    </div>
  );
}

import { useMonitorsQuery, useOverviewQuery } from "@/hooks/useMonitors";
import { useSystemEventsQuery } from "@/hooks/useSystemEvents";

function AdminLayout() {
  const {
    user,
    groups,
    // overview, // Unused
    // fetchOverview, // Replaced by useMonitorsQuery
    addMaintenance,
    isAuthChecked,
    addGroup,
    // addMonitor // Unused
  } = useMonitorStore();
  const { canEdit } = useRole();
  const location = useLocation();
  const navigate = useNavigate();
  const groupId = location.pathname.startsWith('/groups/') ? location.pathname.split('/')[2] : null;
  const overviewQuery = useOverviewQuery();
  useMonitorsQuery(!groupId); // Group pages use their bounded query instead.
  useSystemEventsQuery();
  const safeGroups = groupId
    ? Array.from(new Map([
        ...(overviewQuery.data || []).map(group => [group.id, { id: group.id, name: group.name, monitors: [] }] as const),
        ...(groups || []).map(group => [group.id, { id: group.id, name: group.name, monitors: [] }] as const),
      ]).values())
    : (groups || []);

  // Route Guard
  if (!isAuthChecked) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center text-foreground">
        <div className="flex items-center gap-2">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
          Wait ...
        </div>
      </div>
    )
  }

  if (!user || !user.isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  if (user.role === 'status_viewer') {
    return <Navigate to="/my-pages" replace />;
  }

  const isIncidents = location.pathname.startsWith('/incidents');
  const isMaintenance = location.pathname.startsWith('/maintenance');
  const isSettings = location.pathname.startsWith('/settings');
  const isStatusPages = location.pathname.startsWith('/status-pages');
  const activeGroup = groupId ? safeGroups.find(g => g.id === groupId) : null;
  const monitorRouteId = location.pathname.startsWith('/monitors/') ? location.pathname.split('/')[2] : null;
  const monitorContext = monitorRouteId ? findMonitorWithGroup(safeGroups as Group[], monitorRouteId) : null;

  // Breadcrumbs Generator
  const getBreadcrumbs = () => {
    // Simple path-based breadcrumbs
    const pathSegments = location.pathname.split('/').filter(Boolean);
    const items = [];

    // Home / Dashboard
    if (pathSegments.length === 0 || pathSegments[0] === 'dashboard') {
      return [
        { title: "Dashboard", url: "/dashboard", active: true }
      ];
    }

    // Root is always Dashboard for now (conceptually)
    items.push({ title: "Dashboard", url: "/dashboard", active: false });

    if (isIncidents) items.push({ title: "Incidents", url: "/incidents", active: true });
    else if (isMaintenance) items.push({ title: "Maintenance", url: "/maintenance", active: true });
    else if (isSettings) items.push({ title: "Settings", url: "/settings", active: true });
    else if (isStatusPages) items.push({ title: "Status Pages", url: "/status-pages", active: true });
    else if (activeGroup) {
      items.push({ title: "Groups", url: "/dashboard", active: false }); // Optional intermediate
      items.push({ title: activeGroup.name, url: `/groups/${activeGroup.id}`, active: true });
    }
    else if (monitorRouteId) {
      // /monitors/:id — Dashboard / GroupName / MonitorName. While groups are still
      // loading we fall back to "Monitor" so the breadcrumb still renders something
      // sensible instead of flashing the raw ID.
      if (monitorContext) {
        items.push({ title: monitorContext.group.name, url: `/groups/${monitorContext.group.id}`, active: false });
        items.push({ title: monitorContext.monitor.name, url: location.pathname, active: true });
      } else {
        items.push({ title: "Monitor", url: location.pathname, active: true });
      }
    }

    return items;
  };

  const breadcrumbs = getBreadcrumbs();

  return (
    <SidebarProvider>
      <TooltipProvider delayDuration={0}>
        <AppSidebar groups={safeGroups} />
        <SidebarInset className="bg-background md:rounded-tl-xl md:border-t md:border-l md:border-border/50 overflow-hidden min-h-screen transition-all">
          <header className="flex h-16 shrink-0 items-center gap-2 border-b border-border/40 bg-background/95 px-4 backdrop-blur sticky top-0 z-10">
            <div className="flex items-center gap-2">
              <SidebarTrigger className="-ml-1" />
              <Separator orientation="vertical" className="mr-2 h-4" />
              <Breadcrumb>
                <BreadcrumbList>
                  {breadcrumbs.map((item, index) => (
                    <React.Fragment key={`${item.url}-${index}`}>
                      {index > 0 && <BreadcrumbSeparator />}
                      <BreadcrumbItem>
                        {item.active ? (
                          <BreadcrumbPage>{item.title}</BreadcrumbPage>
                        ) : (
                          <BreadcrumbLink href={item.url} onClick={(e) => {
                            e.preventDefault();
                            navigate(item.url);
                          }}>
                            {item.title}
                          </BreadcrumbLink>
                        )}
                      </BreadcrumbItem>
                    </React.Fragment>
                  ))}
                </BreadcrumbList>
              </Breadcrumb>
            </div>
            <div className="ml-auto flex items-center gap-2">
              {canEdit && (
                isMaintenance ? (
                  <CreateMaintenanceSheet onCreate={addMaintenance} groups={safeGroups} />
                ) : !isIncidents && !isSettings && !isStatusPages ? (
                  <>
                    {!groupId && <CreateGroupSheet onCreate={addGroup} />}
                    <CreateMonitorSheet groups={safeGroups} defaultGroup={activeGroup?.name} />
                  </>
                ) : null
              )}
            </div>
          </header>
          <ScrollArea className="flex-1 p-4 pt-0 h-[calc(100vh-4rem)]">
            <main className="max-w-5xl mx-auto space-y-6 py-6">
              <Routes>
                <Route path="/dashboard" element={<Dashboard />} />
                <Route path="/groups/:groupId" element={<GroupMonitorsPage />} />
                <Route path="/incidents" element={<IncidentsView />} />
                <Route path="/maintenance" element={<MaintenanceView />} />
                <Route path="/monitors/:id" element={<MonitorPage />} />
                <Route path="/digest/:date" element={<DigestRedirect />} />
                <Route path="/notifications" element={<Navigate to="/settings?tab=notifications" replace />} />
                <Route path="/settings" element={<SettingsView />} />
                <Route path="/status-pages" element={<StatusPagesView />} />
                <Route path="/settings/api-keys" element={<Navigate to="/settings?tab=security" replace />} />
                <Route path="/" element={<Navigate to="/dashboard" replace />} />
              </Routes>
            </main>
          </ScrollArea>
        </SidebarInset>
        <Toaster />
      </TooltipProvider>
    </SidebarProvider >
  )
}

import { SetupPage } from "./components/setup/SetupPage";
import { MyPagesView } from "./components/status-viewer/MyPagesView";

const App = () => {
  const { checkAuth, checkSetupStatus, isSetupComplete } = useMonitorStore(); // Use global state
  const [loading, setLoading] = useState(true);

  // Initial Check
  useEffect(() => {
    const init = async () => {
      const done = await checkSetupStatus();
      // state is updated in store by checkSetupStatus
      if (done) {
        await checkAuth();
      }
      setLoading(false);
    };
    init();
  }, [checkSetupStatus, checkAuth]);

  if (loading) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" data-testid="loading-spinner"></div>
      </div>
    );
  }

  // If setup is NOT complete, force setup page for all routes
  if (!isSetupComplete) {
    return (
      <Routes>
        <Route path="/setup" element={<SetupPage />} />
        <Route path="*" element={<Navigate to="/setup" replace />} />
      </Routes>
    )
  }

  return (
    <Routes>
      <Route path="/setup" element={<Navigate to="/login" replace />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/my-pages" element={<MyPagesView />} />
      <Route path="/status/:slug" element={<StatusPage />} />
      <Route path="/*" element={<AdminLayout />} />
    </Routes>
  );
};

export default App;
