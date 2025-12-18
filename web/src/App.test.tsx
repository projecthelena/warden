import { render, screen, waitFor } from '@testing-library/react';
import App from './App';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { useMonitorStore } from './lib/store';

// Mock store
const mockCheckSetupStatus = vi.fn();
const mockCheckAuth = vi.fn();

vi.mock('./lib/store', () => ({
    useMonitorStore: Object.assign(vi.fn(), {
        getState: () => ({
            checkSetupStatus: mockCheckSetupStatus,
            checkAuth: mockCheckAuth
        })
    }),
}));

describe('App Routing', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('renders SetupPage when setup is NOT complete', async () => {
        // Mock state for hook usage
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (useMonitorStore as any).mockReturnValue({
            checkAuth: mockCheckAuth,
            checkSetupStatus: mockCheckSetupStatus,
            isSetupComplete: false,
        });

        // Mock state for direct access
        mockCheckSetupStatus.mockResolvedValue(false);

        render(
            <MemoryRouter initialEntries={['/']}>
                <App />
            </MemoryRouter>
        );

        // Should find "Welcome" from SetupPage
        expect(await screen.findByRole('heading', { name: /Welcome/i })).toBeInTheDocument();
    });

    it('redirects /setup to /login when setup IS complete', async () => {
        // Mock state: Setup IS complete
        mockCheckSetupStatus.mockResolvedValue(true);

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (useMonitorStore as any).mockReturnValue({
            checkAuth: mockCheckAuth,
            checkSetupStatus: mockCheckSetupStatus,
            isSetupComplete: true, // This drives the routing decision
            user: null,
            isAuthChecked: true,
        });

        render(
            <MemoryRouter initialEntries={['/setup']}>
                <App />
            </MemoryRouter>
        );

        // Should NOT find SetupPage "Welcome"
        await waitFor(() => {
            expect(screen.queryByText(/Welcome/i)).not.toBeInTheDocument();
        });
    });
});
