import { beforeEach, describe, expect, it, vi } from 'vitest';

const { mockToast } = vi.hoisted(() => ({ mockToast: vi.fn() }));

vi.mock('@/components/ui/use-toast', () => ({ toast: mockToast }));

import { useMonitorStore } from './store';

const GROUPS = [
    { id: 'g-default', name: 'Default', monitors: [] },
    { id: 'g-prod', name: 'Production', monitors: [] },
];

function mockFetch(response: Partial<Response> & { ok: boolean }) {
    const fetchMock = vi.fn().mockResolvedValue({
        status: response.ok ? 200 : 400,
        text: async () => '',
        json: async () => ({}),
        ...response,
    });
    vi.stubGlobal('fetch', fetchMock);
    return fetchMock;
}

describe('moveMonitor', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        vi.unstubAllGlobals();
        useMonitorStore.setState({ groups: GROUPS });
    });

    it('posts the destination to the monitor group endpoint', async () => {
        const fetchMock = mockFetch({ ok: true });

        const ok = await useMonitorStore.getState().moveMonitor('m1', 'g-prod');

        expect(ok).toBe(true);
        const [url, init] = fetchMock.mock.calls[0];
        expect(url).toBe('/api/monitors/m1/group');
        expect(init.method).toBe('POST');
        expect(init.credentials).toBe('include');
        expect(JSON.parse(init.body)).toEqual({ groupId: 'g-prod' });
    });

    // The whole point of a move over a delete-and-recreate is that the monitor keeps its
    // past, so the confirmation is where we say so.
    it('names the destination group and reassures about the history', async () => {
        mockFetch({ ok: true });

        await useMonitorStore.getState().moveMonitor('m1', 'g-prod');

        expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({
            title: 'Monitor moved',
            description: expect.stringContaining('Production'),
        }));
        expect(mockToast.mock.calls[0][0].description).toMatch(/history/i);
    });

    it('surfaces the server error rather than a generic failure', async () => {
        mockFetch({ ok: false, status: 404, text: async () => JSON.stringify({ error: 'selected group does not exist' }) });

        const ok = await useMonitorStore.getState().moveMonitor('m1', 'g-gone');

        expect(ok).toBe(false);
        expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({
            description: 'selected group does not exist',
            variant: 'destructive',
        }));
    });

    // A body that is not JSON must not turn a failed move into an unhandled rejection.
    it('falls back to a readable message when the error body is not JSON', async () => {
        mockFetch({ ok: false, status: 500, text: async () => '<html>502</html>' });

        const ok = await useMonitorStore.getState().moveMonitor('m1', 'g-prod');

        expect(ok).toBe(false);
        expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({
            description: 'Failed to move monitor.',
            variant: 'destructive',
        }));
    });

    it('reports a network failure instead of throwing', async () => {
        vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
        vi.spyOn(console, 'error').mockImplementation(() => { });

        const ok = await useMonitorStore.getState().moveMonitor('m1', 'g-prod');

        expect(ok).toBe(false);
        expect(mockToast).toHaveBeenCalledWith(expect.objectContaining({ variant: 'destructive' }));
    });
});
