import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { SelectTimezone } from "./select-timezone";

beforeAll(() => {
    vi.stubGlobal("ResizeObserver", class {
        observe() {}
        unobserve() {}
        disconnect() {}
    });
    HTMLElement.prototype.hasPointerCapture = vi.fn(() => false);
    HTMLElement.prototype.scrollIntoView = vi.fn();
});

describe("SelectTimezone", () => {
    it("offers UTC even when the runtime omits it", async () => {
        const onValueChange = vi.fn();
        render(<SelectTimezone value="UTC" onValueChange={onValueChange} />);

        await userEvent.click(screen.getByTestId("timezone-select"));
        await userEvent.click(screen.getByRole("option", { name: "UTC" }));

        expect(onValueChange).toHaveBeenCalledWith("UTC");
    });

    it("selects a searched IANA timezone without submitting its parent form", async () => {
        const onValueChange = vi.fn();
        const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
        render(
            <form onSubmit={onSubmit}>
                <SelectTimezone value="UTC" onValueChange={onValueChange} />
            </form>,
        );

        await userEvent.click(screen.getByTestId("timezone-select"));
        await userEvent.type(screen.getByPlaceholderText("Search timezone..."), "America/Bogota");
        await userEvent.click(screen.getByRole("option", { name: "America/Bogota" }));

        expect(onValueChange).toHaveBeenCalledWith("America/Bogota");
        expect(onSubmit).not.toHaveBeenCalled();
    });
});
