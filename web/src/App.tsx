import { useState, useEffect } from "react";
import { Routes, Route, useParams, useLocation, useNavigate } from "react-router-dom";
import { useEffect as usePageEffect } from "react"; // Alias to avoid conflict if I used it inside Dashboard, effectively just need simple imports
import { AppSidebar } from "./components/layout/AppSidebar";
import { SidebarProvider, SidebarInset, SidebarTrigger } from "./components/ui/sidebar";
import { Separator } from "./components/ui/separator";
import { useMonitorStore } from "./lib/store";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "./components/ui/card";
import { StatusBadge, UptimeHistory } from "./components/ui/monitor-visuals";
import { Button } from "./components/ui/button";
import { Plus, Trash2 } from "lucide-react";
import { CreateMonitorSheet } from "./components/CreateMonitorSheet";
import { CreateGroupSheet } from "./components/CreateGroupSheet";
import { IncidentsView } from "./components/incidents/IncidentsView";
import { CreateIncidentSheet } from "./components/incidents/CreateIncidentSheet";
import { MonitorDetailsSheet } from "./components/MonitorDetailsSheet";
import { NotificationsView } from "./components/notifications/NotificationsView";
import { CreateChannelSheet } from "./components/notifications/CreateChannelSheet";
import { StatusPage } from "./components/status-page/StatusPage";
import { LoginPage } from "./components/auth/LoginPage";
import { SettingsView } from "./components/settings/SettingsView";
import { StatusPagesView } from "./components/status-pages/StatusPagesView";
import { Navigate } from "react-router-dom"; // Import the new sheet
import { Toaster } from "@/components/ui/toaster";

function MonitorCard({ monitor }: { monitor: any }) {
  const [detailsOpen, setDetailsOpen] = useState(false);

  return (
    <>
      <div
        onClick={() => setDetailsOpen(true)}
        className="flex flex-col sm:flex-row items-center justify-between p-6 border rounded-lg bg-card/40 hover:bg-card/60 transition-all gap-6 cursor-pointer group w-full"
      >
        <div className="space-y-1.5 flex-1 min-w-0 mr-4">
          <div className="flex items-center gap-3">
            <div className={`w-2.5 h-2.5 rounded-full shrink-0 ${monitor.status === 'up' ? 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)]' : 'bg-red-500 animate-pulse'}`} />
            <span className="font-semibold text-base group-hover:text-blue-400 transition-colors truncate block" title={monitor.name}>{monitor.name}</span>
          </div>
          <div className="text-sm text-muted-foreground font-mono truncate block opacity-70" title={monitor.url}>{monitor.url}</div>
        </div>

        <div className="flex-none hidden sm:block">
          <UptimeHistory history={monitor.history} />
        </div>

        <div className="flex items-center gap-4 w-[180px] justify-end shrink-0">
          <div className="text-right whitespace-nowrap">
            <div className="text-xs font-mono text-muted-foreground">{monitor.latency}ms</div>
            <div className="text-[10px] text-muted-foreground opacity-50">{monitor.lastCheck}</div>
          </div>
          <StatusBadge status={monitor.status} />
        </div>
      </div>
      <MonitorDetailsSheet monitor={monitor} open={detailsOpen} onOpenChange={setDetailsOpen} />
    </>
  )
}

function MonitorGroup({ group }: { group: any }) {
  const { deleteGroup } = useMonitorStore();

  const handleDelete = () => {
    if (confirm(`Are you sure you want to delete group "${group.name}"?`)) {
      deleteGroup(group.id);
    }
  };

  return (
    <Card className="bg-slate-900/20 border-slate-800">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>{group.name}</CardTitle>
            <CardDescription>ID: {group.id}</CardDescription>
          </div>
          {group.id !== 'default' && (
            <Button variant="ghost" size="icon" onClick={handleDelete} className="text-slate-500 hover:text-red-400 hover:bg-red-950/30">
              <Trash2 className="w-4 h-4" />
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-2">
        {(!group.monitors || group.monitors.length === 0) ? (
          <div className="text-sm text-slate-500 italic py-2">No monitors in this group.</div>
        ) : (
          group.monitors.map((m: any) => (
            <MonitorCard key={m.id} monitor={m} />
          ))
        )}
      </CardContent>
    </Card>
  )
}

function Dashboard() {
  const { groupId } = useParams();
  const { groups, fetchMonitors } = useMonitorStore();
  const safeGroups = groups || [];

  const displayedGroups = groupId
    ? safeGroups.filter(g => g.id === groupId)
    : safeGroups;

  // Poll for updates
  useEffect(() => {
    const interval = setInterval(() => {
      fetchMonitors();
    }, 2000); // 2 seconds

    return () => clearInterval(interval);
  }, [fetchMonitors]);

  return (
    <div className="space-y-6">
      {displayedGroups.map(group => (
        <MonitorGroup key={group.id} group={group} />
      ))}
    </div>
  )
}

function AdminLayout() {
  const { groups, addGroup, addMonitor, addIncident, user, isAuthChecked } = useMonitorStore();
  const location = useLocation();
  const navigate = useNavigate();

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
  const isNotifications = location.pathname === '/notifications';
  const isSettings = location.pathname === '/settings';
  const groupId = location.pathname.startsWith('/groups/') ? location.pathname.split('/')[2] : null;
  const activeGroup = groupId ? safeGroups.find(g => g.id === groupId) : null;

  const pageTitle = isIncidents
    ? "Incidents & Maintenance"
    : isNotifications
      ? "Notifications & Integrations"
      : isSettings
        ? "Settings"
        : (activeGroup ? activeGroup.name : "All Groups");

  const existingGroupNames = safeGroups.map(g => g.name);

  return (
    <SidebarProvider>
      <AppSidebar groups={safeGroups} />
      <SidebarInset>
        <header className="flex h-16 shrink-0 items-center gap-2 border-b border-slate-800 bg-[#020617]/50 px-4 backdrop-blur sticky top-0 z-10 transition-[width,height] ease-linear group-has-[[data-collapsible=icon]]/sidebar-wrapper:h-12">
          <div className="flex items-center gap-2">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 h-4" />
            <div className="font-semibold">{pageTitle}</div>
          </div>
          <div className="ml-auto flex items-center gap-2">
            {isIncidents ? (
              <CreateIncidentSheet onCreate={addIncident} groups={existingGroupNames} />
            ) : isNotifications ? (
              <CreateChannelSheet />
            ) : isSettings ? (
              null
            ) : ( // Dashboard
              <>
                {!activeGroup && <CreateGroupSheet onCreate={addGroup} />}
                <CreateMonitorSheet onCreate={addMonitor} groups={existingGroupNames} />
              </>
            )}
          </div>
        </header>
        <div className="flex-1 overflow-auto p-4 pt-0">
          <main className="max-w-5xl mx-auto space-y-6 py-6">
            <Routes>
              <Route path="/dashboard" element={<Dashboard />} />
              <Route path="/groups/:groupId" element={<Dashboard />} />
              <Route path="/incidents" element={<IncidentsView />} />
              <Route path="/maintenance" element={<IncidentsView />} />
              <Route path="/notifications" element={<NotificationsView />} />
              <Route path="/settings" element={<SettingsView />} />
              <Route path="/status-pages" element={<StatusPagesView />} />
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
            </Routes>
          </main>
        </div>
      </SidebarInset>
      <Toaster />
    </SidebarProvider>
  )
}

const App = () => {
  const { checkAuth } = useMonitorStore();

  useEffect(() => {
    checkAuth();
  }, []);

  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/status/:slug" element={<StatusPage />} />
      <Route path="/*" element={<AdminLayout />} />
    </Routes>
  );
};

export default App;
