import * as React from "react";
import { PanelLeft } from "lucide-react";
import { Slot } from "@radix-ui/react-slot";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type SidebarContextValue = {
  open: boolean;
  setOpen: React.Dispatch<React.SetStateAction<boolean>>;
  openMobile: boolean;
  setOpenMobile: React.Dispatch<React.SetStateAction<boolean>>;
  isMobile: boolean;
  toggleSidebar: () => void;
  state: "expanded" | "collapsed";
};

const SidebarContext = React.createContext<SidebarContextValue | undefined>(undefined);

export const SIDEBAR_WIDTH = "16rem";
export const SIDEBAR_WIDTH_MOBILE = "18rem";
export const SIDEBAR_WIDTH_ICON = "3rem";

export interface SidebarProviderProps extends React.HTMLAttributes<HTMLDivElement> {
  defaultOpen?: boolean;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

import { TooltipProvider } from "@/components/ui/tooltip"

export const SidebarProvider = ({ defaultOpen = true, open: controlledOpen, onOpenChange, children, ...props }: SidebarProviderProps) => {
  const [isMobile, setIsMobile] = React.useState(false);
  const [openState, setOpenState] = React.useState(defaultOpen);
  const open = controlledOpen ?? openState;
  const [openMobile, setOpenMobile] = React.useState(false);

  React.useEffect(() => {
    const query = window.matchMedia("(max-width: 1024px)");
    const updateMedia = () => setIsMobile(query.matches);
    updateMedia();
    query.addEventListener("change", updateMedia);
    return () => query.removeEventListener("change", updateMedia);
  }, []);

  const setOpen = React.useCallback(
    (value: React.SetStateAction<boolean>) => {
      if (controlledOpen !== undefined) {
        const next = typeof value === "function" ? value(open) : value;
        onOpenChange?.(next);
      } else {
        setOpenState(value);
      }
    },
    [controlledOpen, onOpenChange, open]
  );

  const toggleSidebar = React.useCallback(() => {
    if (isMobile) {
      setOpenMobile((prev) => !prev);
    } else {
      setOpen((prev) => !prev);
    }
  }, [isMobile, setOpen]);

  // We add a state so that we can do data-state="expanded" or "collapsed".
  // This makes it easier to style the sidebar with Tailwind classes.
  const state: "expanded" | "collapsed" = open ? "expanded" : "collapsed";

  const value = React.useMemo(
    () => ({
      state,
      open,
      setOpen,
      isMobile,
      openMobile,
      setOpenMobile,
      toggleSidebar,
    }),
    [state, open, setOpen, isMobile, openMobile, setOpenMobile, toggleSidebar]
  );

  return (
    <SidebarContext.Provider value={value}>
      <TooltipProvider delayDuration={0}>
        <div
          style={
            {
              "--sidebar-width": SIDEBAR_WIDTH,
              "--sidebar-width-icon": SIDEBAR_WIDTH_ICON,
              ...props.style,
            } as React.CSSProperties
          }
          className={cn(
            "group/sidebar-wrapper flex min-h-svh w-full has-[[data-variant=inset]]:bg-sidebar",
            props.className
          )}
          {...props}
        >
          {children}
        </div>
      </TooltipProvider>
    </SidebarContext.Provider>
  );
};

export const useSidebar = () => {
  const context = React.useContext(SidebarContext);
  if (!context) {
    throw new Error("useSidebar must be used within a SidebarProvider.");
  }
  return context;
};

export interface SidebarProps extends React.HTMLAttributes<HTMLElement> {
  side?: "left" | "right";
  variant?: "sidebar" | "floating" | "inset";
  collapsible?: "icon" | "offcanvas" | "none";
}

export const Sidebar = React.forwardRef<HTMLElement, SidebarProps>(
  ({ side = "left", variant = "sidebar", collapsible = "icon", className, children, ...props }, ref) => {
    const { open, openMobile, isMobile, setOpenMobile, state } = useSidebar();
    const isCollapsed = !open && !isMobile && collapsible === "icon";
    const shouldOverlay = isMobile || collapsible === "offcanvas";

    return (
      <>
        <aside
          ref={ref}
          data-state={state}
          data-collapsible={collapsible}
          className={cn(
            "group/sidebar z-40 flex flex-col border-r border-[hsl(var(--sidebar-border))] bg-[hsl(var(--sidebar-background))] text-[hsl(var(--sidebar-foreground))] transition-all duration-300",
            shouldOverlay
              ? "fixed inset-y-0 w-[--sidebar-width]"
              : "relative top-0 h-screen lg:sticky lg:self-start",
            "!max-w-[--sidebar-width]",
            isCollapsed && !shouldOverlay ? "w-[--sidebar-width-icon]" : "w-[--sidebar-width]",
            isMobile && (side === "left" ? (openMobile ? "translate-x-0" : "-translate-x-full") : openMobile ? "translate-x-0" : "translate-x-full"),
            !isMobile && "shadow-sm",
            className
          )}
          style={
            {
              "--sidebar-width": SIDEBAR_WIDTH,
              "--sidebar-width-icon": SIDEBAR_WIDTH_ICON,
            } as React.CSSProperties
          }
          {...props}
        >
          {children}
        </aside>
        {isMobile && openMobile && (
          <div className="fixed inset-0 z-30 bg-black/40 lg:hidden" onClick={() => setOpenMobile(false)} />
        )}
      </>
    );
  }
);
Sidebar.displayName = "Sidebar";

export interface SidebarTriggerProps extends React.ComponentPropsWithoutRef<typeof Button> { }

export const SidebarTrigger = ({ className, ...props }: SidebarTriggerProps) => {
  const { toggleSidebar } = useSidebar();
  return (
    <Button variant="ghost" size="icon" onClick={toggleSidebar} className={cn("lg:hidden", className)} {...props}>
      <PanelLeft className="h-4 w-4" />
      <span className="sr-only">Toggle sidebar</span>
    </Button>
  );
};

export const SidebarInset = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("flex flex-1 flex-col", className)} {...props} />
  )
);
SidebarInset.displayName = "SidebarInset";

export const SidebarHeader = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("border-b border-[hsl(var(--sidebar-border))] px-4 py-3", className)} {...props} />
);

export const SidebarFooter = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("mt-auto border-t border-[hsl(var(--sidebar-border))] px-4 py-3", className)} {...props} />
);

export const SidebarContent = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("flex-1 overflow-y-auto px-3 py-4", className)} {...props} />
);

export const SidebarGroup = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("space-y-2", className)} {...props} />
);

export const SidebarGroupLabel = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn(
        "text-xs font-semibold uppercase tracking-wide text-[hsl(var(--sidebar-foreground))/0.6]",
        "group-data-[state=collapsed]/sidebar:hidden",
        className
      )}
      {...props}
    />
  )
);
SidebarGroupLabel.displayName = "SidebarGroupLabel";

export const SidebarGroupContent = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("space-y-1", className)} {...props} />
);

export const SidebarSeparator = ({ className, ...props }: React.HTMLAttributes<HTMLHRElement>) => (
  <hr className={cn("my-3 border-t border-[hsl(var(--sidebar-border))]", className)} {...props} />
);

export const SidebarMenu = ({ className, ...props }: React.HTMLAttributes<HTMLUListElement>) => (
  <ul className={cn("space-y-1", className)} {...props} />
);

export const SidebarMenuItem = ({ className, ...props }: React.LiHTMLAttributes<HTMLLIElement>) => (
  <li className={cn("group/menu-item relative", className)} {...props} />
);

export interface SidebarMenuButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean;
  isActive?: boolean;
}

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

export const SidebarMenuButton = React.forwardRef<
  HTMLButtonElement,
  SidebarMenuButtonProps & {
    size?: "default" | "sm" | "lg"
    variant?: "default" | "outline"
    tooltip?: string | React.ComponentProps<typeof TooltipContent>
  }
>(
  (
    {
      asChild = false,
      isActive = false,
      variant = "default",
      size = "default",
      tooltip,
      className,
      children,
      ...props
    },
    ref
  ) => {
    const Comp = asChild ? Slot : "button"
    const { isMobile, state } = useSidebar()

    const buttonVariants = {
      default: "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
      outline:
        "bg-background shadow-[0_0_0_1px_hsl(var(--sidebar-border))] hover:bg-sidebar-accent hover:text-sidebar-accent-foreground hover:shadow-[0_0_0_1px_hsl(var(--sidebar-accent))]",
    }

    const sizeVariants = {
      default: "h-8 text-sm",
      sm: "h-7 text-xs",
      lg: "h-12 text-sm group-data-[collapsible=icon]/sidebar:!p-0",
    }

    const button = (
      <Comp
        ref={ref}
        data-sidebar="menu-button"
        data-size={size}
        data-active={isActive}
        className={cn(
          "peer/menu-button flex w-full items-center gap-2 overflow-hidden rounded-md p-2 text-left text-sm outline-none ring-sidebar-ring transition-[width,height,padding] hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 active:bg-sidebar-accent active:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-50 group-has-[[data-sidebar=menu-action]]/menu-item:pr-8 aria-disabled:pointer-events-none aria-disabled:opacity-50 data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-sidebar-accent-foreground data-[state=open]:hover:bg-sidebar-accent data-[state=open]:hover:text-sidebar-accent-foreground group-data-[collapsible=icon]/sidebar:group-data-[state=collapsed]/sidebar:!size-8 group-data-[collapsible=icon]/sidebar:group-data-[state=collapsed]/sidebar:!p-2 [&>span:last-child]:truncate [&>svg]:size-4 [&>svg]:shrink-0",
          "text-sidebar-foreground",
          buttonVariants[variant],
          sizeVariants[size],
          className
        )}
        {...props}
      >
        {children}
      </Comp>
    )

    if (!tooltip) {
      return button
    }

    if (typeof tooltip === "string") {
      tooltip = {
        children: tooltip,
      }
    }

    return (
      <Tooltip>
        <TooltipTrigger asChild>{button}</TooltipTrigger>
        <TooltipContent
          side="right"
          align="center"
          hidden={state !== "collapsed" && !isMobile}
          {...tooltip}
        />
      </Tooltip>
    )
  }
)
SidebarMenuButton.displayName = "SidebarMenuButton"

export const SidebarMenuBadge = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    className={cn(
      "ml-auto rounded-full bg-[hsl(var(--sidebar-accent))] px-2 py-0.5 text-[11px] font-semibold text-[hsl(var(--sidebar-accent-foreground))]",
      className
    )}
    {...props}
  />
);

export const SidebarMenuSkeleton = ({ className, showIcon }: { className?: string; showIcon?: boolean }) => (
  <div className={cn("flex items-center gap-2 px-3 py-2", className)}>
    {showIcon && <div className="h-4 w-4 rounded-full bg-[hsl(var(--sidebar-border))]" />}
    <div className="h-4 flex-1 rounded bg-[hsl(var(--sidebar-border))]" />
  </div>
);

export const SidebarMenuAction = React.forwardRef<
  HTMLButtonElement,
  React.ButtonHTMLAttributes<HTMLButtonElement> & {
    asChild?: boolean
    showOnHover?: boolean
  }
>(({ className, asChild = false, showOnHover = false, ...props }, ref) => {
  const Comp = asChild ? Slot : "button"

  return (
    <Comp
      ref={ref}
      className={cn(
        "absolute right-1 top-1.5 flex aspect-square h-8 w-8 items-center justify-center rounded-md p-0.5 text-sidebar-foreground transition-transform hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring [&>svg]:size-4 [&>svg]:shrink-0",
        // Increases the hit area of the button on hover.
        "after:absolute after:-inset-2 after:md:hidden",
        "peer-data-[size=sm]/menu-button:top-1",
        "peer-data-[size=default]/menu-button:top-1.5",
        "peer-data-[size=lg]/menu-button:top-2.5",
        "group-data-[collapsible=icon]/sidebar:hidden",
        showOnHover &&
        "group-focus-within/menu-item:opacity-100 group-hover/menu-item:opacity-100 data-[state=open]:opacity-100 peer-data-[active=true]/menu-button:text-sidebar-accent-foreground md:opacity-0",
        className
      )}
      {...props}
    />
  )
})
SidebarMenuAction.displayName = "SidebarMenuAction"

export const SidebarMenuSub = ({ className, ...props }: React.HTMLAttributes<HTMLUListElement>) => (
  <ul className={cn("ml-6 space-y-1 border-l border-[hsl(var(--sidebar-border))] pl-3 text-sm", className)} {...props} />
);

export const SidebarMenuSubItem = SidebarMenuItem;

export const SidebarMenuSubButton = SidebarMenuButton;

export const SidebarRail = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn("hidden w-2 bg-transparent lg:block", className)} {...props} />
);
