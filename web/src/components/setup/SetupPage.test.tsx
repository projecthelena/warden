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
            <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
                <SetupPage />
            </BrowserRouter>
        );
        expect(screen.getByText(/Welcome/i)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /Get Started/i })).toBeInTheDocument();
    });

    it('navigates to the next step when Get Started is clicked', async () => {
        render(
            <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
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

        mockUseMonitorStore.mockReturnValue({
            performSetup: mockPerformSetup,
            login: vi.fn(),
        });

        // Mock static getState for fallback check
        (mockUseMonitorStore as unknown as { getState: ReturnType<typeof vi.fn> }).getState = vi.fn().mockReturnValue({
            checkSetupStatus: vi.fn().mockResolvedValue(true)
        });

        // Mock window.location
        Object.defineProperty(window, 'location', {
            value: { href: '' },
            writable: true
        });

        render(
            <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
                <SetupPage />
            </BrowserRouter>
        );

        // Step 1: Welcome -> Get Started
        fireEvent.click(screen.getByRole('button', { name: /Get Started/i }));
        await waitFor(() => expect(screen.getByText(/Create Admin Account/i)).toBeInTheDocument());

        // Step 2: Fill in credentials and submit
        fireEvent.change(screen.getByTestId('setup-username-input'), { target: { value: 'testadmin' } });
        fireEvent.change(screen.getByTestId('setup-password-input'), { target: { value: 'Pass123!@' } });

        // Click Launch Dashboard (button becomes enabled after valid password)
        fireEvent.click(screen.getByRole('button', { name: /Launch Dashboard/i }));

        await waitFor(() => {
            expect(mockPerformSetup).toHaveBeenCalledWith(expect.objectContaining({
                username: 'testadmin',
                password: 'Pass123!@'
            }));
            expect(window.location.href).toBe('/');
        });
    });
});
