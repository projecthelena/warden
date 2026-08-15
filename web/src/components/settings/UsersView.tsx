import { useState, useEffect, useCallback } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useToast } from "@/components/ui/use-toast";
import { MoreHorizontal, Trash2, Users, FileText, KeyRound } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { useMonitorStore } from "@/lib/store";

interface UserDTO {
    id: number;
    username: string;
    role: string;
    email?: string;
    ssoProvider?: string;
    avatar?: string;
    displayName?: string;
    createdAt: string;
}

const ROLE_LABELS: Record<string, string> = {
    admin: "Admin",
    editor: "Editor",
    viewer: "Viewer",
    status_viewer: "Status Viewer",
};

const ROLE_COLORS: Record<string, string> = {
    admin: "destructive",
    editor: "default",
    viewer: "secondary",
    status_viewer: "outline",
};

interface StatusPageOption {
    id: number;
    slug: string;
    title: string;
    public: boolean;
    enabled: boolean;
}

export function UsersView() {
    const { toast } = useToast();
    const currentUser = useMonitorStore((s) => s.user);
    const [users, setUsers] = useState<UserDTO[]>([]);
    const [deleteId, setDeleteId] = useState<number | null>(null);
    const [resetUser, setResetUser] = useState<UserDTO | null>(null);
    const [newPassword, setNewPassword] = useState("");
    const [resetting, setResetting] = useState(false);
    const [managePagesUser, setManagePagesUser] = useState<UserDTO | null>(null);
    const [allStatusPages, setAllStatusPages] = useState<StatusPageOption[]>([]);
    const [selectedPageIds, setSelectedPageIds] = useState<Set<number>>(new Set());
    const [savingPages, setSavingPages] = useState(false);
    const [userPageCounts, setUserPageCounts] = useState<Record<number, number>>({});

    const fetchPageCounts = useCallback(async (userList: UserDTO[]) => {
        const statusViewers = userList.filter(u => u.role === "status_viewer");
        const counts: Record<number, number> = {};
        await Promise.all(statusViewers.map(async (u) => {
            try {
                const res = await fetch(`/api/users/${u.id}/status-pages`, { credentials: "include" });
                if (res.ok) {
                    const data = await res.json();
                    counts[u.id] = (data.statusPageIds || []).length;
                }
            } catch { /* ignore */ }
        }));
        setUserPageCounts(counts);
    }, []);

    const fetchUsers = useCallback(async () => {
        try {
            const res = await fetch("/api/users", { credentials: "include" });
            if (res.ok) {
                const data = await res.json();
                const userList = data.users || [];
                setUsers(userList);
                fetchPageCounts(userList);
            }
        } catch (e) {
            console.error("Failed to fetch users:", e);
        }
    }, [fetchPageCounts]);

    useEffect(() => {
        fetchUsers();
    }, [fetchUsers]);

    useEffect(() => {
        const handler = () => fetchUsers();
        window.addEventListener("users-updated", handler);
        return () => window.removeEventListener("users-updated", handler);
    }, [fetchUsers]);

    const handleRoleChange = async (userId: number, newRole: string) => {
        try {
            const res = await fetch(`/api/users/${userId}/role`, {
                method: "PATCH",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ role: newRole }),
                credentials: "include",
            });
            if (res.ok) {
                toast({ title: "Role Updated", description: "User role has been changed." });
                fetchUsers();
            } else {
                const data = await res.json().catch(() => ({ error: "Failed to update role" }));
                toast({ title: "Error", description: data.error, variant: "destructive" });
            }
        } catch {
            toast({ title: "Error", description: "Failed to update role.", variant: "destructive" });
        }
    };

    const handleDelete = async () => {
        if (deleteId === null) return;
        try {
            const res = await fetch(`/api/users/${deleteId}`, {
                method: "DELETE",
                credentials: "include",
            });
            if (res.ok) {
                toast({ title: "User Deleted", description: "User has been removed." });
                fetchUsers();
            } else {
                const data = await res.json().catch(() => ({ error: "Failed to delete user" }));
                toast({ title: "Error", description: data.error, variant: "destructive" });
            }
        } catch {
            toast({ title: "Error", description: "Failed to delete user.", variant: "destructive" });
        }
        setDeleteId(null);
    };

    const handleResetPassword = async () => {
        if (!resetUser) return;
        setResetting(true);
        try {
            const res = await fetch(`/api/users/${resetUser.id}/password`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ password: newPassword }),
                credentials: "include",
            });
            if (res.ok) {
                toast({ title: "Password Reset", description: `New password set for ${resetUser.displayName || resetUser.username}. Their sessions were signed out.` });
                setResetUser(null);
                setNewPassword("");
            } else {
                const data = await res.json().catch(() => ({ error: "Failed to reset password" }));
                toast({ title: "Error", description: data.error, variant: "destructive" });
            }
        } catch {
            toast({ title: "Error", description: "Failed to reset password.", variant: "destructive" });
        }
        setResetting(false);
    };

    const openManagePages = async (user: UserDTO) => {
        setManagePagesUser(user);
        try {
            // Fetch all status pages and current assignments in parallel
            const [pagesRes, assignRes] = await Promise.all([
                fetch("/api/status-pages", { credentials: "include" }),
                fetch(`/api/users/${user.id}/status-pages`, { credentials: "include" }),
            ]);
            if (pagesRes.ok) {
                const pagesData = await pagesRes.json();
                // Only show saved pages (id > 0) that are enabled
                const saved = (pagesData.pages || []).filter((p: StatusPageOption) => p.id > 0 && p.enabled);
                setAllStatusPages(saved);
            }
            if (assignRes.ok) {
                const assignData = await assignRes.json();
                setSelectedPageIds(new Set(assignData.statusPageIds || []));
            }
        } catch {
            toast({ title: "Error", description: "Failed to load status pages.", variant: "destructive" });
        }
    };

    const handleSavePages = async () => {
        if (!managePagesUser) return;
        setSavingPages(true);
        try {
            const res = await fetch(`/api/users/${managePagesUser.id}/status-pages`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ statusPageIds: Array.from(selectedPageIds) }),
                credentials: "include",
            });
            if (res.ok) {
                toast({ title: "Pages Updated", description: "Status page assignments saved." });
                setManagePagesUser(null);
                fetchUsers();
            } else {
                const data = await res.json().catch(() => ({ error: "Failed to save" }));
                toast({ title: "Error", description: data.error, variant: "destructive" });
            }
        } catch {
            toast({ title: "Error", description: "Failed to save page assignments.", variant: "destructive" });
        } finally {
            setSavingPages(false);
        }
    };

    const togglePageId = (id: number) => {
        setSelectedPageIds(prev => {
            const next = new Set(prev);
            if (next.has(id)) {
                next.delete(id);
            } else {
                next.add(id);
            }
            return next;
        });
    };

    return (
        <Card>
            <CardHeader>
                <CardTitle>User Management</CardTitle>
                <CardDescription>Manage users and their roles.</CardDescription>
            </CardHeader>
            <CardContent>
                {users.length > 0 ? (
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>User</TableHead>
                                <TableHead>Role</TableHead>
                                <TableHead>Created</TableHead>
                                <TableHead className="w-[50px]"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {users.map((u) => {
                                const isSelf = currentUser?.username === u.username;
                                return (
                                    <TableRow key={u.id}>
                                        <TableCell>
                                            <div className="flex flex-col">
                                                <span className="font-medium">
                                                    {u.displayName || u.username}
                                                    {isSelf && <span className="text-muted-foreground ml-1">(you)</span>}
                                                </span>
                                                {u.email && (
                                                    <span className="text-xs text-muted-foreground">{u.email}</span>
                                                )}
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <div className="flex items-center gap-2">
                                                {isSelf ? (
                                                    <Badge variant={ROLE_COLORS[u.role] as "default" | "secondary" | "destructive" | "outline"}>
                                                        {ROLE_LABELS[u.role] || u.role}
                                                    </Badge>
                                                ) : (
                                                    <Select value={u.role} onValueChange={(v) => handleRoleChange(u.id, v)}>
                                                        <SelectTrigger className="w-[140px] h-8">
                                                            <SelectValue />
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                            <SelectItem value="admin">Admin</SelectItem>
                                                            <SelectItem value="editor">Editor</SelectItem>
                                                            <SelectItem value="viewer">Viewer</SelectItem>
                                                            <SelectItem value="status_viewer">Status Viewer</SelectItem>
                                                        </SelectContent>
                                                    </Select>
                                                )}
                                                {u.role === "status_viewer" && (
                                                    <span className="text-xs text-muted-foreground">
                                                        {userPageCounts[u.id] ?? 0} {(userPageCounts[u.id] ?? 0) === 1 ? "page" : "pages"}
                                                    </span>
                                                )}
                                            </div>
                                        </TableCell>
                                        <TableCell className="text-muted-foreground">
                                            {new Date(u.createdAt).toLocaleDateString()}
                                        </TableCell>
                                        <TableCell>
                                            {!isSelf && (
                                                <DropdownMenu>
                                                    <DropdownMenuTrigger asChild>
                                                        <Button variant="ghost" size="icon" className="h-8 w-8">
                                                            <MoreHorizontal className="h-4 w-4" />
                                                        </Button>
                                                    </DropdownMenuTrigger>
                                                    <DropdownMenuContent align="end">
                                                        {u.role === "status_viewer" && (
                                                            <DropdownMenuItem onClick={() => openManagePages(u)}>
                                                                <FileText className="h-4 w-4 mr-2" />
                                                                Manage Pages
                                                            </DropdownMenuItem>
                                                        )}
                                                        <DropdownMenuItem
                                                            data-testid={`reset-password-${u.username}`}
                                                            onClick={() => { setResetUser(u); setNewPassword(""); }}
                                                        >
                                                            <KeyRound className="h-4 w-4 mr-2" />
                                                            Reset Password
                                                        </DropdownMenuItem>
                                                        <DropdownMenuItem
                                                            className="text-destructive focus:text-destructive"
                                                            onClick={() => setDeleteId(u.id)}
                                                        >
                                                            <Trash2 className="h-4 w-4 mr-2" />
                                                            Delete User
                                                        </DropdownMenuItem>
                                                    </DropdownMenuContent>
                                                </DropdownMenu>
                                            )}
                                        </TableCell>
                                    </TableRow>
                                );
                            })}
                        </TableBody>
                    </Table>
                ) : (
                    <div className="flex flex-col items-center justify-center p-12 border border-dashed border-border rounded-lg text-muted-foreground">
                        <Users className="w-12 h-12 mb-4 opacity-50" />
                        <h3 className="text-lg font-medium text-foreground mb-1">No Users</h3>
                        <p className="text-sm">No users found.</p>
                    </div>
                )}
            </CardContent>

            <Dialog open={managePagesUser !== null} onOpenChange={(open) => !open && setManagePagesUser(null)}>
                <DialogContent className="sm:max-w-[450px]">
                    <DialogHeader>
                        <DialogTitle>Manage Status Pages</DialogTitle>
                        <DialogDescription>
                            Assign status pages to {managePagesUser?.displayName || managePagesUser?.username}.
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4 space-y-3 max-h-[300px] overflow-y-auto">
                        {allStatusPages.length === 0 ? (
                            <p className="text-sm text-muted-foreground text-center py-4">
                                No enabled status pages found. Enable a status page first.
                            </p>
                        ) : (
                            allStatusPages.map((page) => (
                                <label
                                    key={page.id}
                                    className="flex items-center gap-3 p-2 rounded-md hover:bg-accent/50 cursor-pointer"
                                >
                                    <Checkbox
                                        checked={selectedPageIds.has(page.id)}
                                        onCheckedChange={() => togglePageId(page.id)}
                                    />
                                    <div className="flex-1 min-w-0">
                                        <div className="text-sm font-medium truncate">{page.title}</div>
                                        <div className="text-xs text-muted-foreground">/{page.slug}</div>
                                    </div>
                                    <Badge variant={page.public ? "secondary" : "outline"} className="text-xs shrink-0">
                                        {page.public ? "Public" : "Private"}
                                    </Badge>
                                </label>
                            ))
                        )}
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setManagePagesUser(null)}>Cancel</Button>
                        <Button onClick={handleSavePages} disabled={savingPages}>
                            {savingPages ? "Saving..." : "Save"}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            <Dialog open={resetUser !== null} onOpenChange={(open) => { if (!open) { setResetUser(null); setNewPassword(""); } }}>
                <DialogContent className="sm:max-w-[425px]">
                    <DialogHeader>
                        <DialogTitle>Reset Password</DialogTitle>
                        <DialogDescription>
                            Set a new password for {resetUser?.displayName || resetUser?.username}. This signs them out of all sessions.
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4 space-y-2">
                        <Label htmlFor="new-password">New password</Label>
                        <Input
                            id="new-password"
                            data-testid="reset-password-input"
                            type="password"
                            value={newPassword}
                            onChange={(e) => setNewPassword(e.target.value)}
                            placeholder="At least 8 chars, a number and a symbol"
                        />
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => { setResetUser(null); setNewPassword(""); }}>Cancel</Button>
                        <Button data-testid="reset-password-submit" onClick={handleResetPassword} disabled={resetting || newPassword.length < 8}>
                            {resetting ? "Resetting..." : "Reset Password"}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            <AlertDialog open={deleteId !== null} onOpenChange={(open) => !open && setDeleteId(null)}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>Delete User?</AlertDialogTitle>
                        <AlertDialogDescription>
                            This will permanently delete the user and all their sessions.
                            This action cannot be undone.
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                            Delete
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </Card>
    );
}
