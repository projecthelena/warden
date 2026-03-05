import { useEffect, useState } from "react";
import { ThemeContext } from "./theme-context";

export type Theme = "dark" | "light" | "system";

const STORAGE_KEY = "warden-theme";

function getSystemTheme(): "dark" | "light" {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyThemeToDOM(theme: Theme): "dark" | "light" {
    const resolved = theme === "system" ? getSystemTheme() : theme;
    const root = document.documentElement;
    root.classList.remove("dark", "light");
    root.classList.add(resolved);
    return resolved;
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
    const [theme, setThemeState] = useState<Theme>(() => {
        return (localStorage.getItem(STORAGE_KEY) as Theme) || "system";
    });

    const [resolvedTheme, setResolvedTheme] = useState<"dark" | "light">(() => {
        const stored = (localStorage.getItem(STORAGE_KEY) as Theme) || "system";
        return stored === "system" ? getSystemTheme() : (stored as "dark" | "light");
    });

    const setTheme = (t: Theme) => {
        localStorage.setItem(STORAGE_KEY, t);
        setThemeState(t);
        const resolved = applyThemeToDOM(t);
        setResolvedTheme(resolved);
    };

    // Apply on mount (sync with localStorage / anti-FOUC backup)
    useEffect(() => {
        const resolved = applyThemeToDOM(theme);
        setResolvedTheme(resolved);
    }, []); // eslint-disable-line react-hooks/exhaustive-deps

    // Listen for system preference changes
    useEffect(() => {
        if (theme !== "system") return;
        const mq = window.matchMedia("(prefers-color-scheme: dark)");
        const handler = () => {
            const resolved = applyThemeToDOM("system");
            setResolvedTheme(resolved);
        };
        mq.addEventListener("change", handler);
        return () => mq.removeEventListener("change", handler);
    }, [theme]);

    return (
        <ThemeContext.Provider value={{ theme, setTheme, resolvedTheme }}>
            {children}
        </ThemeContext.Provider>
    );
}
