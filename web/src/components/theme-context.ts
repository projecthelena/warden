import { createContext } from "react";
import type { Theme } from "./theme-provider";

export interface ThemeContextValue {
    theme: Theme;
    setTheme: (theme: Theme) => void;
    resolvedTheme: "dark" | "light";
}

export const ThemeContext = createContext<ThemeContextValue | undefined>(undefined);
