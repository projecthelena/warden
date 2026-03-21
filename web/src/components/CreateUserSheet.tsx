import { useState } from "react";
import { Plus } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetFooter,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
    SheetClose,
} from "@/components/ui/sheet";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/components/ui/use-toast";

export function CreateUserSheet() {
    const [open, setOpen] = useState(false);
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [role, setRole] = useState("viewer");
    const [usernameError, setUsernameError] = useState(false);
    const [passwordError, setPasswordError] = useState(false);
    const [isSubmitting, setIsSubmitting] = useState(false);
    const { toast } = useToast();

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        let hasError = false;
        if (!username.trim()) {
            setUsernameError(true);
            hasError = true;
        }
        if (!password || password.length < 8) {
            setPasswordError(true);
            toast({ title: "Invalid Password", description: "Password must be at least 8 characters.", variant: "destructive" });
            hasError = true;
        }
        if (hasError) return;

        setIsSubmitting(true);
        try {
            const res = await fetch("/api/users", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ username: username.trim(), password, role }),
                credentials: "include",
            });

            if (res.ok) {
                toast({ title: "User Created", description: `User "${username.trim()}" has been created.` });
                setUsername("");
                setPassword("");
                setRole("viewer");
                setOpen(false);
                window.dispatchEvent(new Event("users-updated"));
            } else {
                const data = await res.json().catch(() => ({ error: "Failed to create user" }));
                toast({ title: "Error", description: data.error, variant: "destructive" });
            }
        } catch {
            toast({ title: "Error", description: "Failed to create user.", variant: "destructive" });
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button className="gap-2" size="sm" data-testid="create-user-trigger">
                    <Plus className="w-4 h-4" /> New User
                </Button>
            </SheetTrigger>
            <SheetContent className="sm:max-w-[450px] overflow-y-auto">
                <SheetHeader>
                    <SheetTitle>Create New User</SheetTitle>
                    <SheetDescription>
                        Add a new user to the system and assign them a role.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label htmlFor="create-username">Username</Label>
                        <Input
                            id="create-username"
                            placeholder="e.g. john"
                            value={username}
                            onChange={(e) => {
                                setUsername(e.target.value);
                                if (usernameError) setUsernameError(false);
                            }}
                            className={cn(usernameError && "border-red-500 focus-visible:ring-red-500")}
                            required
                            data-testid="create-user-username-input"
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="create-password">Password</Label>
                        <Input
                            id="create-password"
                            type="password"
                            placeholder="Minimum 8 characters"
                            value={password}
                            onChange={(e) => {
                                setPassword(e.target.value);
                                if (passwordError) setPasswordError(false);
                            }}
                            className={cn(passwordError && "border-red-500 focus-visible:ring-red-500")}
                            required
                            data-testid="create-user-password-input"
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="create-role">Role</Label>
                        <Select value={role} onValueChange={setRole}>
                            <SelectTrigger data-testid="create-user-role-select">
                                <SelectValue placeholder="Select a role" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="admin" className="cursor-pointer">Admin</SelectItem>
                                <SelectItem value="editor" className="cursor-pointer">Editor</SelectItem>
                                <SelectItem value="viewer" className="cursor-pointer">Viewer</SelectItem>
                                <SelectItem value="status_viewer" className="cursor-pointer">Status Viewer</SelectItem>
                            </SelectContent>
                        </Select>
                        <p className="text-[0.8rem] text-muted-foreground">
                            {role === "admin" && "Full access to all features and settings."}
                            {role === "editor" && "Can create and edit monitors, groups, and events."}
                            {role === "viewer" && "Read-only access to the dashboard."}
                            {role === "status_viewer" && "Can only view assigned status pages."}
                        </p>
                    </div>
                    <SheetFooter className="mt-4">
                        <SheetClose asChild>
                            <Button variant="outline" className="mr-2">Cancel</Button>
                        </SheetClose>
                        <Button type="submit" disabled={isSubmitting} data-testid="create-user-submit-btn">
                            {isSubmitting ? "Creating..." : "Create User"}
                        </Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
