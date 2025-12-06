import { useState } from "react";
import { Routes, Route, useParams, useLocation } from "react-router-dom";
import { AppSidebar } from "./components/layout/AppSidebar";
import { SidebarProvider, SidebarInset } from "./components/ui/sidebar";
import { useMonitorStore } from "./lib/store";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "./components/ui/card";
import { StatusBadge, UptimeHistory } from "./components/ui/monitor-visuals";
import { Button } from "./components/ui/button";
import { Plus } from "lucide-react";
import { CreateMonitorSheet } from "./components/CreateMonitorSheet";
import { CreateGroupSheet } from "./components/CreateGroupSheet";
import { IncidentsView } from "./components/incidents/IncidentsView";
import { CreateIncidentSheet } from "./components/incidents/CreateIncidentSheet";
import { StatusPage } from "./components/status-page/StatusPage";
import { MonitorDetailsSheet } from "./components/MonitorDetailsSheet"; // Import the new sheet

function MonitorCard({ monitor }: { monitor: any }) {
  const [detailsOpen, setDetailsOpen] = useState(false);

  return (
    <>
      <div
        onClick={() => setDetailsOpen(true)}
        className="flex flex-col sm:flex-row items-start sm:items-center justify-between p-4 border rounded-lg bg-card/40 hover:bg-card/60 transition-colors gap-4 cursor-pointer group"
      >
        <div className="space-y-1 min-w-[200px]">
          <div className="flex items-center gap-2">
            <div className={`w-2 h-2 rounded-full ${monitor.status === 'up' ? 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)]' : 'bg-red-500 animate-pulse'}`} />
            <span className="font-medium text-sm group-hover:text-blue-400 transition-colors">{monitor.name}</span>
          </div>
          <div className="text-xs text-muted-foreground font-mono">{monitor.url}</div>
        </div>

        <div className="flex-1 w-full sm:w-auto px-4">
          <UptimeHistory history={monitor.history} />
        </div>

        <div className="flex items-center gap-4 min-w-[180px] justify-end">
          <div className="text-right">
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
  return (
    <Card className="bg-slate-900/20 border-slate-800">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>{group.name}</CardTitle>
            <CardDescription>ID: {group.id}</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-2">
        {group.monitors.length === 0 ? (
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
  const { groups } = useMonitorStore();

  const displayedGroups = groupId
    ? groups.filter(g => g.id === groupId)
    : groups;

  return (
    <div className="space-y-6">
      {displayedGroups.map(group => (
        <MonitorGroup key={group.id} group={group} />
      ))}
    </div>
  )
}

function AdminLayout() {
  const { groups, addGroup, addMonitor, addIncident } = useMonitorStore();
  const location = useLocation();
  const params = useParams(); // Note: inside Routes, useParams works for the matched route

  // We need to resolve title and actions based on current path
  const isIncidents = location.pathname === '/incidents';
  const groupId = location.pathname.startsWith('/groups/') ? location.pathname.split('/')[2] : null;
  const activeGroup = groupId ? groups.find(g => g.id === groupId) : null;

  const pageTitle = isIncidents
    ? "Incidents & Maintenance"
    : (activeGroup ? activeGroup.name : "All Groups");

  const existingGroupNames = groups.map(g => g.name);

  return (
    <SidebarProvider>
      <div className="flex min-h-screen w-full bg-[#020617] text-slate-100">
        <AppSidebar groups={groups} />
        <SidebarInset className="flex-1 flex flex-col min-w-0">
          <header className="flex h-14 items-center gap-4 border-b border-slate-800 bg-[#020617]/50 px-6 backdrop-blur sticky top-0 z-10">
            <div className="font-semibold">{pageTitle}</div>
            <div className="ml-auto flex items-center gap-2">
              {isIncidents ? (
                <CreateIncidentSheet onCreate={addIncident} groups={existingGroupNames} />
              ) : (
                <>
                  {!activeGroup && <CreateGroupSheet onCreate={addGroup} />}
                  <CreateMonitorSheet onCreate={addMonitor} groups={existingGroupNames} />
                </>
              )}
            </div>
          </header>
          <main className="flex-1 overflow-auto p-6">
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/groups/:groupId" element={<Dashboard />} />
              <Route path="/incidents" element={<IncidentsView />} />
            </Routes>
          </main>
        </SidebarInset>
      </div>
    </SidebarProvider>
  )
}

const App = () => {
  return (
    <Routes>
      <Route path="/status" element={<StatusPage />} />
      <Route path="/*" element={<AdminLayout />} />
    </Routes>
  );
};

export default App;
