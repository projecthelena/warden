import { useState } from "react";
import { AppSidebar } from "./components/layout/AppSidebar";
import { SidebarProvider, SidebarInset } from "./components/ui/sidebar";
import { useMonitorStore } from "./lib/store";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "./components/ui/card";
import { StatusBadge, UptimeHistory } from "./components/ui/monitor-visuals";
import { Button } from "./components/ui/button";
import { Plus } from "lucide-react";

function MonitorCard({ monitor }: { monitor: any }) {
  return (
    <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between p-4 border rounded-lg bg-card/40 hover:bg-card/60 transition-colors gap-4">
      <div className="space-y-1 min-w-[200px]">
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${monitor.status === 'up' ? 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)]' : 'bg-red-500 animate-pulse'}`} />
          <span className="font-medium text-sm">{monitor.name}</span>
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
  )
}

function ProjectGroup({ project }: { project: any }) {
  return (
    <Card className="bg-slate-900/20 border-slate-800">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>{project.name}</CardTitle>
            <CardDescription>ID: {project.id}</CardDescription>
          </div>
          <Button variant="outline" size="sm" className="h-7 text-xs border-slate-700 bg-slate-800/50">
            <Plus className="w-3 h-3 mr-1" /> Add Monitor
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-2">
        {project.monitors.map((m: any) => (
          <MonitorCard key={m.id} monitor={m} />
        ))}
      </CardContent>
    </Card>
  )
}

const App = () => {
  const { projects } = useMonitorStore();
  const [activeProject, setActiveProject] = useState<string | null>(null);

  const displayedProjects = activeProject
    ? projects.filter(p => p.id === activeProject)
    : projects;

  return (
    <SidebarProvider>
      <div className="flex min-h-screen w-full bg-[#020617] text-slate-100">
        <AppSidebar
          projects={projects}
          activeProject={activeProject}
          onSelectProject={setActiveProject}
        />
        <SidebarInset className="flex-1 flex flex-col min-w-0">
          <header className="flex h-14 items-center gap-4 border-b border-slate-800 bg-[#020617]/50 px-6 backdrop-blur sticky top-0 z-10">
            <div className="font-semibold">{activeProject ? projects.find(p => p.id === activeProject)?.name : "All Projects"}</div>
            <div className="ml-auto flex items-center gap-2">
              <span className="flex h-2 w-2 rounded-full bg-green-500 animate-pulse"></span>
              <span className="text-xs text-muted-foreground">Live Updates</span>
            </div>
          </header>
          <main className="flex-1 overflow-auto p-6 space-y-6">
            {displayedProjects.map(project => (
              <ProjectGroup key={project.id} project={project} />
            ))}
          </main>
        </SidebarInset>
      </div>
    </SidebarProvider>
  );
};

export default App;
