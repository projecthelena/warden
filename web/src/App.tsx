import { useState, useEffect } from "react";
import React from "react";
import { Routes, Route, useParams, useLocation, useNavigate, Navigate } from "react-router-dom";
import { AppSidebar } from "./components/layout/AppSidebar";
import { SidebarProvider, SidebarInset, SidebarTrigger } from "./components/ui/sidebar";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "./components/ui/separator";

import { useMonitorStore, Group, OverviewGroup } from "./lib/store";
import { Card, CardContent, CardHeader, CardTitle } from "./components/ui/card";
import { Button } from "./components/ui/button";
import { Trash2, ChevronRight } from "lucide-react";
import { MonitorCard } from "./components/MonitorCard";
import { CreateMonitorSheet } from "./components/CreateMonitorSheet";
import { CreateGroupSheet } from "./components/CreateGroupSheet";
import { CreateMaintenanceSheet } from "./components/incidents/CreateMaintenanceSheet";
import { MaintenanceView } from "./components/incidents/MaintenanceView";
import { IncidentsView } from "./components/incidents/IncidentsView";
import { NotificationsView } from "./components/notifications/NotificationsView";
import { CreateChannelSheet } from "./components/notifications/CreateChannelSheet";
import { StatusPage } from "./components/status-page/StatusPage";
import { LoginPage } from "./components/auth/LoginPage";
import { SettingsView } from "./components/settings/SettingsView";
import { StatusPagesView } from "./components/status-pages/StatusPagesView";
import { APIKeysPage } from "./components/settings/APIKeysPage";
import { CreateAPIKeySheet } from "./components/settings/CreateAPIKeySheet";
import { Toaster } from "@/components/ui/toaster";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"



function MonitorGroup({ group }: { group: Group }) {
  const { deleteGroup } = useMonitorStore();

  const handleDelete = () => {
    if (confirm(`Are you sure you want to delete group "${group.name}"?`)) {
      deleteGroup(group.id);
    }
  };

  return (
    <Card className="border-border bg-card">
      <CardHeader className="p-4 pb-2">
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>{group.name}</CardTitle>
          </div>
          {group.id !== 'default' && (
            <Button variant="ghost" size="icon" onClick={handleDelete} className="text-muted-foreground hover:text-destructive">
              <Trash2 className="w-4 h-4" />
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-2 p-4 pt-0">
        {(!group.monitors || group.monitors.length === 0) ? (
          <div className="text-sm text-slate-500 italic py-2">No monitors in this group.</div>
        ) : (
          group.monitors.map((m) => (
            <MonitorCard key={m.id} monitor={m} />
          ))
        )}
      </CardContent>
    </Card>
  )
}

// New Lightweight Group Card for Overview
// New Lightweight Group Card for Overview (Status Page Style)
function GroupOverviewCard({ group }: { group: OverviewGroup }) {
  const navigate = useNavigate();

  const statusColor =
    group.status === 'up' ? 'bg-green-500' :
      group.status === 'degraded' ? 'bg-yellow-500' : 'bg-red-500';

  const statusText =
    group.status === 'up' ? 'Operational' :
      group.status === 'degraded' ? 'Degraded' : 'Down';

  const statusTextColor =
    group.status === 'up' ? 'text-green-500' :
      group.status === 'degraded' ? 'text-yellow-500' : 'text-red-500';

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
            {group.status !== 'up' && (
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
  const { groupId } = useParams();
  const { groups, overview, fetchMonitors, fetchOverview } = useMonitorStore();
  const safeGroups = groups || [];

  // Poll for updates based on view
  useEffect(() => {
    // Initial fetch
    if (groupId) {
      fetchMonitors(groupId);
    } else {
      fetchOverview();
    }

    const interval = setInterval(() => {
      if (groupId) {
        fetchMonitors(groupId);
      } else {
        fetchOverview();
      }
    }, 60000); // 60 seconds

    return () => clearInterval(interval);
  }, [fetchMonitors, fetchOverview, groupId]);

  if (!groupId) {
    // Overview Mode
    const safeOverview = overview || [];
    const downGroups = safeOverview.filter(g => g.status === 'down').length;
    const degradedGroups = safeOverview.filter(g => g.status === 'degraded').length;
    const isHealthy = downGroups === 0 && degradedGroups === 0;

    return (
      <div className="space-y-6">
        <div>
          <h2 className="text-xl font-semibold tracking-tight text-foreground">
            {isHealthy ? "All Systems Operational" : "System Issues Detected"}
          </h2>
          <p className={`text-sm ${isHealthy ? 'text-muted-foreground' : 'text-red-400'}`}>
            {isHealthy
              ? `Monitoring ${safeOverview.length} check groups. Everything looks good.`
              : `${downGroups} groups down, ${degradedGroups} degraded.`}
          </p>
        </div>

        {safeOverview.length === 0 && (
          <div className="text-center text-muted-foreground py-10 border border-border rounded-xl bg-card">
            No groups found. Create one to get started.
          </div>
        )}
        <div className="grid gap-3">
          {safeOverview.map(group => (
            <GroupOverviewCard key={group.id} group={group} />
          ))}
        </div>
      </div>
    );
  }

  // Detail Mode (Single Group)
  // We filter from 'groups' state which should now contain only this group's data (populated by fetchMonitors(groupId))
  // However, fetchMonitors replaces the whole 'groups' array.
  return (
    <div className="space-y-8">
      {safeGroups.map(group => (
        <MonitorGroup key={group.id} group={group} />
      ))}
    </div>
  )
}

function AdminLayout() {
  const {
    user,
    groups,
    overview,
    fetchOverview,
    addMaintenance,
    isAuthChecked,
    addGroup,
    addMonitor
  } = useMonitorStore();
  const location = useLocation();
  const navigate = useNavigate();

  // Ensure overview is loaded for Sidebar
  useEffect(() => {
    fetchOverview();
  }, [fetchOverview]);

  const safeGroups = groups || [];

  // Route Guard
  if (!isAuthChecked) {
    return (
      <div className="min-h-screen bg-[#020617] flex items-center justify-center text-slate-100">
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

  const isIncidents = location.pathname === '/incidents';
  const isMaintenance = location.pathname === '/maintenance';
  const isNotifications = location.pathname === '/notifications';
  const isSettings = location.pathname === '/settings';
  const isStatusPages = location.pathname === '/status-pages';
  const isApiKeys = location.pathname === '/api-keys';
  const groupId = location.pathname.startsWith('/groups/') ? location.pathname.split('/')[2] : null;
  const activeGroup = groupId ? safeGroups.find(g => g.id === groupId) : null;

  const existingGroupNames = (overview || groups || []).map(g => g.name);

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
    else if (isNotifications) items.push({ title: "Notifications", url: "/notifications", active: true });
    else if (isSettings) items.push({ title: "Settings", url: "/settings", active: true });
    else if (isStatusPages) items.push({ title: "Status Pages", url: "/status-pages", active: true });
    else if (isApiKeys) items.push({ title: "API Keys", url: "/api-keys", active: true });
    else if (activeGroup) {
      items.push({ title: "Groups", url: "/dashboard", active: false }); // Optional intermediate
      items.push({ title: activeGroup.name, url: `/groups/${activeGroup.id}`, active: true });
    }

    return items;
  };

  const breadcrumbs = getBreadcrumbs();

  return (
    <SidebarProvider>
      <AppSidebar groups={overview || safeGroups} />
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
            {isIncidents ? (
              null
            ) : isMaintenance ? (
              <CreateMaintenanceSheet onCreate={addMaintenance} groups={existingGroupNames} />
            ) : isNotifications ? (
              <CreateChannelSheet />
            ) : isSettings ? (
              null
            ) : isApiKeys ? (
              <CreateAPIKeySheet />
            ) : isStatusPages ? (
              null
            ) : ( // Dashboard
              <>
                {!groupId && <CreateGroupSheet onCreate={addGroup} />}
                <CreateMonitorSheet onCreate={addMonitor} groups={existingGroupNames} defaultGroup={activeGroup?.name} />
              </>
            )}
          </div>
        </header>
        <ScrollArea className="flex-1 p-4 pt-0 h-[calc(100vh-4rem)]">
          <main className="max-w-5xl mx-auto space-y-6 py-6">
            <Routes>
              <Route path="/dashboard" element={<Dashboard />} />
              <Route path="/groups/:groupId" element={<Dashboard />} />
              <Route path="/incidents" element={<IncidentsView />} />
              <Route path="/maintenance" element={<MaintenanceView />} />
              <Route path="/notifications" element={<NotificationsView />} />
              <Route path="/settings" element={<SettingsView />} />
              <Route path="/status-pages" element={<StatusPagesView />} />
              <Route path="/api-keys" element={<APIKeysPage />} />
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
            </Routes>
          </main>
        </ScrollArea>
      </SidebarInset>
      <Toaster />
    </SidebarProvider>
  )
}

import { SetupPage } from "./components/setup/SetupPage";

const App = () => {
  const { checkAuth, checkSetupStatus, isSetupComplete } = useMonitorStore(); // Use global state
  const [loading, setLoading] = useState(true);

  // Initial Check
  useEffect(() => {
    const init = async () => {
      const done = await checkSetupStatus();
      // state is updated in store by checkSetupStatus
      if (done) {
        checkAuth();
      }
      setLoading(false);
    };
    init();
  }, [checkSetupStatus, checkAuth]);

  if (loading) {
    return (
      <div className="min-h-screen bg-[#020617] flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  // If setup is NOT complete, force setup page for all routes except static assets if any
  if (!isSetupComplete) {
    return (
      <Routes>
        <Route path="*" element={<SetupPage />} />
      </Routes>
    )
  }

  return (
    <Routes>
      <Route path="/setup" element={<Navigate to="/login" replace />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/status/:slug" element={<StatusPage />} />
      <Route path="/*" element={<AdminLayout />} />
    </Routes>
  );
};

export default App;
