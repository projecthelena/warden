import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { MarkdownContent, MarkdownEditor } from "./markdown";

describe("MarkdownContent", () => {
    it("renders supported formatting without treating HTML as markup", () => {
        render(<MarkdownContent>{"**Bold**\n\n- First\n\n<script>alert('no')</script>"}</MarkdownContent>);

        expect(screen.getByText("Bold").tagName).toBe("STRONG");
        expect(screen.getByText("First").tagName).toBe("LI");
        expect(screen.getByText(/<script>/)).toBeInTheDocument();
        expect(document.querySelector("script")).not.toBeInTheDocument();
    });
});

describe("MarkdownEditor", () => {
    it("applies toolbar formatting and previews the result", () => {
        let value = "maintenance";
        render(<MarkdownEditor value={value} onChange={(next) => { value = next; }} />);
        const textarea = screen.getByRole("textbox");
        fireEvent.select(textarea, { target: { selectionStart: 0, selectionEnd: value.length } });
        fireEvent.click(screen.getByRole("button", { name: "Bold" }));

        expect(value).toBe("**maintenance**");
    });
});
