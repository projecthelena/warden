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
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"



import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"

import { useDeleteGroupMutation } from "@/hooks/useMonitors";

function MonitorGroup({ group }: { group: Group }) {
  const mutation = useDeleteGroupMutation();
  const navigate = useNavigate();

  const handleDelete = async () => {
    await mutation.mutateAsync(group.id);
    navigate('/dashboard');
  };

  return (
    <Card className="border-border bg-card">
      <CardHeader className="p-4 pb-2">
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>{group.name}</CardTitle>
          </div>
          {group.id !== 'default' && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="ghost" size="icon" className="text-muted-foreground hover:text-destructive" data-testid="delete-group-trigger">
                  <Trash2 className="w-4 h-4" />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This action cannot be undone. This will permanently delete the group
                    "{group.name}" and all monitors associated with it.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90" data-testid="delete-group-confirm">
                    Delete
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-2 p-4 pt-0">
        {(!group.monitors || group.monitors.length === 0) ? (
          <div className="text-sm text-slate-500 italic py-2">No monitors in this group.</div>
        ) : (
          group.monitors.map((m) => (
            <MonitorCard key={m.id} monitor={m} groupId={group.id} />
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
  const { groupId } = useParams();
  const { groups, incidents, fetchIncidents } = useMonitorStore();
  const safeGroups = groups || [];

  useEffect(() => {
    fetchIncidents();
  }, [fetchIncidents]);

  // Filter groups if a specific group ID is selected
  const displayGroups = groupId
    ? safeGroups.filter(g => g.id === groupId)
    : safeGroups;

  // Derive overview stats from the full groups data (Client-side aggregation)
  const derivedOverview = safeGroups.map(group => {
    // Check for maintenance first
    const now = new Date();
    const isMaintenance = incidents.some(i =>
      i.type === 'maintenance' &&
      i.status !== 'completed' &&
      i.affectedGroups.includes(group.id) &&
      new Date(i.startTime) <= now &&
      (!i.endTime || new Date(i.endTime) > now)
    );

    if (isMaintenance) {
      return { ...group, status: 'maintenance' as const };
    }

    let status: 'up' | 'down' | 'degraded' | 'maintenance' = 'up';
    if (!group.monitors || group.monitors.length === 0) {
      status = 'up';
    } else {
      const anyDown = group.monitors.some(m => m.status === 'down');
      const anyDegraded = group.monitors.some(m => m.status === 'degraded');
      if (anyDown) status = 'down';
      else if (anyDegraded) status = 'degraded';
    }
    return { ...group, status };
  });

  if (!groupId) {
    // Overview Mode
    const downGroups = derivedOverview.filter(g => g.status === 'down').length;
    const degradedGroups = derivedOverview.filter(g => g.status === 'degraded').length;
    const isHealthy = downGroups === 0 && degradedGroups === 0;

    return (
      <div className="space-y-6">
        <div>
          <h2 className="text-xl font-semibold tracking-tight text-foreground">
            {isHealthy ? "All Systems Operational" : "System Issues Detected"}
          </h2>
          <p className={`text-sm ${isHealthy ? 'text-muted-foreground' : 'text-red-400'}`}>
            {isHealthy
              ? `Monitoring ${derivedOverview.length} check groups. Everything looks good.`
              : `${downGroups} groups down, ${degradedGroups} degraded.`}
          </p>
        </div>

        {derivedOverview.length === 0 && (
          <div className="text-center text-muted-foreground py-10 border border-border rounded-xl bg-card">
            No groups found. Create one to get started.
          </div>
        )}
        <div className="grid gap-3">
          {derivedOverview.map(group => (
            <GroupOverviewCard key={group.id} group={group} />
          ))}
        </div>
      </div>
    );
  }

  // Detail Mode (Single Group)
  return (
    <div className="space-y-8">
      {displayGroups.map(group => (
        <MonitorGroup key={group.id} group={group} />
      ))}
    </div>
  )
}

import { useMonitorsQuery } from "@/hooks/useMonitors";
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

  useMonitorsQuery(); // Handles polling
  useSystemEventsQuery(); // Handles polling events

  const location = useLocation();
  const navigate = useNavigate();

  console.log('DEBUG: App Render Path:', location.pathname);

  // Ensure overview is loaded for Sidebar
  // useEffect removed as query handles it

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

  const isIncidents = location.pathname.startsWith('/incidents');
  const isMaintenance = location.pathname.startsWith('/maintenance');
  const isNotifications = location.pathname.startsWith('/notifications');
  const isSettings = location.pathname.startsWith('/settings') && !location.pathname.startsWith('/settings/api-keys');
  const isStatusPages = location.pathname.startsWith('/status-pages');
  const isApiKeys = location.pathname.startsWith('/settings/api-keys') || location.pathname.startsWith('/api-keys'); // Backwards compat or just use new
  const groupId = location.pathname.startsWith('/groups/') ? location.pathname.split('/')[2] : null;
  const activeGroup = groupId ? safeGroups.find(g => g.id === groupId) : null;

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
    else if (isApiKeys) items.push({ title: "API Keys", url: "/settings/api-keys", active: true });
    else if (activeGroup) {
      items.push({ title: "Groups", url: "/dashboard", active: false }); // Optional intermediate
      items.push({ title: activeGroup.name, url: `/groups/${activeGroup.id}`, active: true });
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
              {isIncidents ? (
                null
              ) : isMaintenance ? (
                <CreateMaintenanceSheet onCreate={addMaintenance} groups={safeGroups} />
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
                  <CreateMonitorSheet groups={safeGroups} defaultGroup={activeGroup?.name} />
                </>
              )}
              <Button data-testid="debug-button-always">DebugAlways</Button>
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
                <Route path="/settings/api-keys" element={<APIKeysPage />} />
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
      <div className="min-h-screen bg-[#020617] flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" data-testid="loading-spinner"></div>
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
