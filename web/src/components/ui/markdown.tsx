/* Hallmark · component: markdown editor · genre: modern-minimal · theme: existing Warden tokens
 * states: default · hover · focus · active · disabled · loading · error · success
 * contrast: pass
 */
import { useRef, useState } from "react";
import { Bold, Italic, Link, List } from "lucide-react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

interface MarkdownContentProps {
    children: string;
    className?: string;
}

export function MarkdownContent({ children, className }: MarkdownContentProps) {
    return (
        <div className={cn("space-y-2 break-words text-sm leading-relaxed", className)}>
            <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                    a: ({ children: linkChildren, ...props }) => <a {...props} className="text-primary underline underline-offset-4" target="_blank" rel="noreferrer">{linkChildren}</a>,
                    p: ({ children: paragraphChildren }) => <p>{paragraphChildren}</p>,
                    ul: ({ children: listChildren }) => <ul className="ml-5 list-disc space-y-1">{listChildren}</ul>,
                    ol: ({ children: listChildren }) => <ol className="ml-5 list-decimal space-y-1">{listChildren}</ol>,
                    code: ({ children: codeChildren }) => <code className="rounded bg-muted px-1 py-0.5 font-mono text-[0.9em]">{codeChildren}</code>,
                }}
            >
                {children}
            </ReactMarkdown>
        </div>
    );
}

interface MarkdownEditorProps {
    value: string;
    onChange: (value: string) => void;
    id?: string;
    placeholder?: string;
}

export function MarkdownEditor({ value, onChange, id, placeholder }: MarkdownEditorProps) {
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const [tab, setTab] = useState("write");

    const wrapSelection = (before: string, after = before, fallback = "text") => {
        const textarea = textareaRef.current;
        const start = textarea?.selectionStart ?? value.length;
        const end = textarea?.selectionEnd ?? value.length;
        const selection = value.slice(start, end) || fallback;
        onChange(`${value.slice(0, start)}${before}${selection}${after}${value.slice(end)}`);
        requestAnimationFrame(() => {
            textarea?.focus();
            textarea?.setSelectionRange(start + before.length, start + before.length + selection.length);
        });
    };

    const prefixLines = (prefix: string) => {
        const textarea = textareaRef.current;
        const start = textarea?.selectionStart ?? value.length;
        const end = textarea?.selectionEnd ?? value.length;
        const lineStart = value.lastIndexOf("\n", Math.max(0, start - 1)) + 1;
        const selection = value.slice(lineStart, end) || "List item";
        const replacement = selection.split("\n").map((line) => `${prefix}${line}`).join("\n");
        onChange(`${value.slice(0, lineStart)}${replacement}${value.slice(end)}`);
        requestAnimationFrame(() => textarea?.focus());
    };

    return (
        <Tabs value={tab} onValueChange={setTab}>
            <div className="flex items-center justify-between gap-2 border-b">
                <TabsList className="h-9 bg-transparent p-0">
                    <TabsTrigger value="write" className="h-9 rounded-none border-b-2 border-transparent px-3 data-[state=active]:border-primary data-[state=active]:bg-transparent">Write</TabsTrigger>
                    <TabsTrigger value="preview" className="h-9 rounded-none border-b-2 border-transparent px-3 data-[state=active]:border-primary data-[state=active]:bg-transparent">Preview</TabsTrigger>
                </TabsList>
                {tab === "write" && (
                    <div className="flex items-center gap-1" aria-label="Markdown formatting">
                        <Button type="button" variant="ghost" size="icon" className="h-8 w-8" onClick={() => wrapSelection("**", "**")} aria-label="Bold"><Bold className="h-4 w-4" /></Button>
                        <Button type="button" variant="ghost" size="icon" className="h-8 w-8" onClick={() => wrapSelection("_", "_")} aria-label="Italic"><Italic className="h-4 w-4" /></Button>
                        <Button type="button" variant="ghost" size="icon" className="h-8 w-8" onClick={() => prefixLines("- ")} aria-label="Bulleted list"><List className="h-4 w-4" /></Button>
                        <Button type="button" variant="ghost" size="icon" className="h-8 w-8" onClick={() => wrapSelection("[", "](https://)", "link text")} aria-label="Link"><Link className="h-4 w-4" /></Button>
                    </div>
                )}
            </div>
            <TabsContent value="write" className="mt-2">
                <Textarea ref={textareaRef} id={id} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="min-h-32 resize-y" data-testid="maintenance-description-input" />
                <p className="mt-1.5 text-xs text-muted-foreground">Markdown supported</p>
            </TabsContent>
            <TabsContent value="preview" className="mt-2 min-h-32 rounded-md border bg-muted/20 p-3">
                {value.trim() ? <MarkdownContent>{value}</MarkdownContent> : <p className="text-sm text-muted-foreground">Nothing to preview yet.</p>}
            </TabsContent>
        </Tabs>
    );
}
