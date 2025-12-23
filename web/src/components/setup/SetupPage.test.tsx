import { SetupPage } from './SetupPage';
import { BrowserRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

// Define mock using vi.hoisted to ensure it's available for the factory
const { mockUseMonitorStore } = vi.hoisted(() => {
    return { mockUseMonitorStore: vi.fn() };
});

// Mock dependencies
vi.mock('../../lib/store', () => ({
    useMonitorStore: mockUseMonitorStore,
}));

describe('SetupPage', () => {
    // Reset the mock before each test
    beforeEach(() => {
        mockUseMonitorStore.mockReturnValue({
            performSetup: vi.fn(),
            login: vi.fn(),
        });
    });

    it('renders the welcome step initially', () => {
        render(
            <BrowserRouter>
                <SetupPage />
            </BrowserRouter>
        );
        expect(screen.getByText(/Welcome/i)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /Get Started/i })).toBeInTheDocument();
    });

    it('navigates to the next step when Get Started is clicked', async () => {
        render(
            <BrowserRouter>
                <SetupPage />
            </BrowserRouter>
        );
        fireEvent.click(screen.getByRole('button', { name: /Get Started/i }));

        await waitFor(() => {
            expect(screen.getByText(/Create Admin Account/i)).toBeInTheDocument();
        });
    });

    it('completes the full setup flow', async () => {
        const mockPerformSetup = vi.fn().mockResolvedValue({ success: true });
        const mockLogin = vi.fn().mockResolvedValue({ success: true });

        mockUseMonitorStore.mockReturnValue({
            performSetup: mockPerformSetup,
            login: mockLogin,
        });

        // Mock static getState for fallback check
        mockUseMonitorStore.getState = vi.fn().mockReturnValue({
            checkSetupStatus: vi.fn().mockResolvedValue(true)
        });

        // Mock window.location
        Object.defineProperty(window, 'location', {
            value: { href: '' },
            writable: true
        });

        render(
            <BrowserRouter>
                <SetupPage />
            </BrowserRouter>
        );

        // Step 1: Welcome -> Get Started
        fireEvent.click(screen.getByRole('button', { name: /Get Started/i }));
        await waitFor(() => expect(screen.getByText(/Create Admin Account/i)).toBeInTheDocument());

        // Step 2: Form Input
        fireEvent.change(screen.getByPlaceholderText(/e.g. admin/i), { target: { value: 'admin' } });
        fireEvent.change(screen.getByPlaceholderText(/Min 8 chars/i), { target: { value: 'Pass123!@' } });

        fireEvent.click(screen.getByRole('button', { name: /Continue/i }));
        await waitFor(() => expect(screen.getByText(/Select Timezone/i)).toBeInTheDocument());

        // Step 3: Timezone (Just click continue, default is selected)
        fireEvent.click(screen.getByRole('button', { name: /Continue/i }));
        await waitFor(() => expect(screen.getByText(/Almost Done/i)).toBeInTheDocument());

        // Step 4: Submit
        fireEvent.click(screen.getByRole('button', { name: /Launch Dashboard/i }));

        await waitFor(() => {
            expect(mockPerformSetup).toHaveBeenCalledWith(expect.objectContaining({
                username: 'admin',
                password: 'Pass123!@'
            }));
            expect(mockLogin).toHaveBeenCalledWith('admin', 'Pass123!@');
            expect(window.location.href).toBe('/');
        });
    });
});
