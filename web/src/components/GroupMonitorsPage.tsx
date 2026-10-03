/* Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V4
 * genre: modern-minimal · macrostructure: Workbench · design-system: design.md · designed-as-app
 * contrast: pass · responsive: 320/375/414/768 pass · motion: functional opacity only
 */
import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { AlertCircle, ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, Pause, Search, Server, Trash2, X } from "lucide-react";
import { useDeleteGroupMutation, useGroupMonitorsQuery, type GroupMonitorStatusFilter } from "@/hooks/useMonitors";
import { useMonitorStore } from "@/lib/store";
import { useRole } from "@/hooks/useRole";
import { MonitorCard } from "@/components/MonitorCard";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
    AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription,
    AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { cn } from "@/lib/utils";

const PAGE_SIZES = [25, 50, 100];
const STATUS_FILTERS: Array<{ value: GroupMonitorStatusFilter; label: string; icon?: typeof AlertCircle }> = [
    { value: "all", label: "All" },
    { value: "operational", label: "Operational", icon: Server },
    { value: "issues", label: "Issues", icon: AlertCircle },
    { value: "paused", label: "Paused", icon: Pause },
];

function positiveInt(value: string | null, fallback: number) {
    const parsed = Number(value);
    return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function statusFilter(value: string | null): GroupMonitorStatusFilter {
    return value === "operational" || value === "issues" || value === "paused" ? value : "all";
}

export function GroupMonitorsPage() {
    const { groupId } = useParams<{ groupId: string }>();
    const [params, setParams] = useSearchParams();
    const navigate = useNavigate();
    const { canEdit } = useRole();
    const { fetchIncidents } = useMonitorStore();
    const deleteGroup = useDeleteGroupMutation();
    const page = positiveInt(params.get("page"), 1);
    const requestedSize = positiveInt(params.get("pageSize"), 25);
    const pageSize = PAGE_SIZES.includes(requestedSize) ? requestedSize : 25;
    const search = params.get("search") || "";
    const status = statusFilter(params.get("status"));
    const [searchInput, setSearchInput] = useState(search);
    const query = useGroupMonitorsQuery(groupId, { page, pageSize, search, status });

    useEffect(() => { fetchIncidents(); }, [fetchIncidents]);
    useEffect(() => { setSearchInput(search); }, [search]);
    useEffect(() => {
        const timer = window.setTimeout(() => {
            const nextSearch = searchInput.trim();
            if (nextSearch === search) return;
            const next = new URLSearchParams(params);
            if (nextSearch) next.set("search", nextSearch);
            else next.delete("search");
            next.delete("page");
            setParams(next, { replace: true });
        }, 300);
        return () => window.clearTimeout(timer);
    }, [params, search, searchInput, setParams]);

    useEffect(() => {
        const totalPages = query.data?.pagination.totalPages;
        if (totalPages && page > totalPages) {
            const next = new URLSearchParams(params);
            next.set("page", String(totalPages));
            setParams(next, { replace: true });
        }
    }, [page, params, query.data?.pagination.totalPages, setParams]);

    const setQuery = (updates: Record<string, string | null>) => {
        const next = new URLSearchParams(params);
        Object.entries(updates).forEach(([key, value]) => value ? next.set(key, value) : next.delete(key));
        setParams(next, { replace: true });
    };

    const counts = query.data?.counts;
    const group = query.data?.group;
    const pagination = query.data?.pagination;
    const visibleRange = useMemo(() => {
        if (!pagination || pagination.total === 0) return "0 monitors";
        const first = (pagination.page - 1) * pagination.pageSize + 1;
        const last = Math.min(first + pagination.pageSize - 1, pagination.total);
        return `${first}–${last} of ${pagination.total}`;
    }, [pagination]);

    if (query.isPending) return <GroupPageSkeleton />;
    if (query.isError) {
        return <div className="grid min-h-72 place-items-center rounded-xl border border-dashed border-border bg-card/30 px-5 text-center">
            <div className="space-y-3"><AlertCircle className="mx-auto h-6 w-6 text-rose-400" /><div><h1 className="font-medium">Could not load this group</h1><p className="mt-1 text-sm text-muted-foreground">{query.error.message}</p></div><Button variant="outline" onClick={() => query.refetch()}>Try again</Button></div>
        </div>;
    }
    if (!group || !pagination || !counts) return null;

    const monitors = group.monitors || [];
    const hasFilters = Boolean(search || status !== "all");

    return (
        <div className="space-y-5" data-testid="group-monitors-page">
            <header className="rounded-xl border border-border bg-card p-4 shadow-none sm:p-6">
                <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                    <div className="min-w-0 space-y-2">
                        <h1 className="min-w-0 [overflow-wrap:anywhere] text-2xl font-semibold tracking-tight sm:text-3xl">{group.name}</h1>
                        <p className="text-sm text-muted-foreground">{counts.all} monitor{counts.all === 1 ? "" : "s"} · {counts.operational} operational · {counts.issues} issue{counts.issues === 1 ? "" : "s"}</p>
                    </div>
                    {canEdit && group.id !== "default" && group.id !== "g-default" && <AlertDialog>
                        <AlertDialogTrigger asChild><Button variant="outline" className="self-start whitespace-nowrap text-muted-foreground hover:text-destructive" data-testid="delete-group-trigger"><Trash2 className="mr-2 h-4 w-4" />Delete group</Button></AlertDialogTrigger>
                        <AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Delete {group.name}?</AlertDialogTitle><AlertDialogDescription>This permanently deletes the group and every monitor in it, including their history.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" data-testid="delete-group-confirm" onClick={async () => { await deleteGroup.mutateAsync(group.id); navigate("/dashboard"); }}>Delete group</AlertDialogAction></AlertDialogFooter></AlertDialogContent>
                    </AlertDialog>}
                </div>
            </header>

            <section aria-label="Monitor filters" className="rounded-xl border border-border bg-card p-3 sm:p-4">
                <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
                    <div className="relative min-w-0 flex-1 xl:max-w-md">
                        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input value={searchInput} onChange={event => setSearchInput(event.target.value)} placeholder="Search name, target, or monitor ID" aria-label="Search monitors" className="h-10 pl-9 pr-10" />
                        {searchInput && <Button type="button" variant="ghost" size="icon" className="absolute right-0 top-0 h-10 w-10" aria-label="Clear monitor search" onClick={() => setSearchInput("")}><X className="h-4 w-4" /></Button>}
                    </div>
                    <div className="flex flex-wrap gap-2" role="group" aria-label="Filter monitors by status">
                        {STATUS_FILTERS.map(filter => {
                            const count = counts[filter.value === "all" ? "all" : filter.value];
                            const Icon = filter.icon;
                            return <Button key={filter.value} variant={status === filter.value ? "secondary" : "ghost"} size="sm" className={cn("h-10 whitespace-nowrap", status === filter.value && "border border-border bg-secondary")} aria-pressed={status === filter.value} onClick={() => setQuery({ status: filter.value === "all" ? null : filter.value, page: null })}>{Icon && <Icon className="mr-2 h-4 w-4" />}{filter.label}<span className="ml-2 tabular-nums text-muted-foreground">{count}</span></Button>;
                        })}
                    </div>
                </div>
            </section>

            <section aria-live="polite" aria-busy={query.isFetching} className={cn("space-y-2 transition-opacity", query.isFetching && query.isPlaceholderData && "opacity-60")}>
                {monitors.length ? monitors.map(monitor => <MonitorCard key={monitor.id} monitor={monitor} groupId={group.id} />) : <div className="grid min-h-48 place-items-center rounded-xl border border-dashed border-border bg-card/30 px-5 text-center"><div className="space-y-2"><Search className="mx-auto h-6 w-6 text-muted-foreground" /><h2 className="font-medium">{hasFilters ? "No monitors match these filters" : "No monitors in this group"}</h2><p className="text-sm text-muted-foreground">{hasFilters ? "Clear the search or choose another status." : "Create a monitor to start checking this group."}</p>{hasFilters && <Button variant="outline" onClick={() => { setSearchInput(""); setQuery({ search: null, status: null, page: null }); }}>Clear filters</Button>}</div></div>}
            </section>

            <footer className="flex flex-col gap-3 rounded-xl border border-border bg-card px-3 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-4">
                <div className="flex items-center justify-between gap-3 text-sm text-muted-foreground sm:justify-start"><span className="tabular-nums">{visibleRange}</span><div className="flex items-center gap-2"><span className="whitespace-nowrap">Per page</span><Select value={String(pageSize)} onValueChange={value => setQuery({ pageSize: value === "25" ? null : value, page: null })}><SelectTrigger className="h-9 w-20" aria-label="Monitors per page"><SelectValue /></SelectTrigger><SelectContent>{PAGE_SIZES.map(size => <SelectItem key={size} value={String(size)}>{size}</SelectItem>)}</SelectContent></Select></div></div>
                <div className="flex items-center justify-between gap-2 sm:justify-end"><span className="mr-1 whitespace-nowrap text-sm text-muted-foreground">Page {pagination.totalPages ? pagination.page : 0} of {pagination.totalPages}</span><Button variant="outline" size="icon" className="hidden h-10 w-10 sm:inline-flex" aria-label="First page" disabled={page <= 1} onClick={() => setQuery({ page: null })}><ChevronsLeft className="h-4 w-4" /></Button><Button variant="outline" size="icon" className="h-10 w-10" aria-label="Previous page" disabled={page <= 1} onClick={() => setQuery({ page: page - 1 === 1 ? null : String(page - 1) })}><ChevronLeft className="h-4 w-4" /></Button><Button variant="outline" size="icon" className="h-10 w-10" aria-label="Next page" disabled={!pagination.totalPages || page >= pagination.totalPages} onClick={() => setQuery({ page: String(page + 1) })}><ChevronRight className="h-4 w-4" /></Button><Button variant="outline" size="icon" className="hidden h-10 w-10 sm:inline-flex" aria-label="Last page" disabled={!pagination.totalPages || page >= pagination.totalPages} onClick={() => setQuery({ page: String(pagination.totalPages) })}><ChevronsRight className="h-4 w-4" /></Button></div>
            </footer>
        </div>
    );
}

function GroupPageSkeleton() {
    return <div className="space-y-5"><Skeleton className="h-36 w-full rounded-xl" /><Skeleton className="h-20 w-full rounded-xl" /><div className="space-y-2">{Array.from({ length: 8 }, (_, index) => <Skeleton key={index} className="h-24 w-full rounded-lg" />)}</div><Skeleton className="h-16 w-full rounded-xl" /></div>;
}
