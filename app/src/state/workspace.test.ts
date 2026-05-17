import { describe, it, expect, beforeEach, vi } from 'vitest';

vi.mock('../ipc/client', () => ({
  probe: vi.fn(),
}));
import { probe } from '../ipc/client';
import { useWorkspace, resetWorkspaceForTest } from './workspace';

const mockProbe = vi.mocked(probe);

describe('workspace store', () => {
  beforeEach(() => {
    resetWorkspaceForTest();
    mockProbe.mockReset();
  });

  it('starts in EMPTY state', () => {
    expect(useWorkspace.getState().status).toBe('EMPTY');
  });

  it('loadFile transitions EMPTY -> FILE_LOADED -> PROBING -> READY on success', async () => {
    const result = {
      id: 'A', path: '/x.mp4', filename: 'x.mp4', sizeBytes: 1,
      container: { format: 'mp4', longName: '' }, audio: null,
      compatibility: { supported: false, issues: ['no audio'], warnings: [] },
    };
    mockProbe.mockResolvedValue(result);
    const transitions: string[] = [];
    const unsub = useWorkspace.subscribe((s) => transitions.push(s.status));
    await useWorkspace.getState().loadFile('/x.mp4');
    unsub();
    expect(transitions).toContain('FILE_LOADED');
    expect(transitions).toContain('PROBING');
    expect(useWorkspace.getState().status).toBe('READY');
    expect(useWorkspace.getState().result).toEqual(result);
  });

  it('loadFile transitions to ERROR on probe rejection', async () => {
    mockProbe.mockRejectedValue({ code: 'FILE_NOT_FOUND', message: 'missing' });
    await useWorkspace.getState().loadFile('/x.mp4');
    expect(useWorkspace.getState().status).toBe('ERROR');
    expect(useWorkspace.getState().error?.code).toBe('FILE_NOT_FOUND');
  });

  it('retry replays loadFile on the same path', async () => {
    mockProbe
      .mockRejectedValueOnce({ code: 'FFPROBE_FAILURE', message: 'transient' })
      .mockResolvedValueOnce({
        id: 'B', path: '/x.mp4', filename: 'x.mp4', sizeBytes: 1,
        container: { format: 'mp4', longName: '' }, audio: null,
        compatibility: { supported: false, issues: [], warnings: [] },
      });
    await useWorkspace.getState().loadFile('/x.mp4');
    expect(useWorkspace.getState().status).toBe('ERROR');
    await useWorkspace.getState().retry();
    expect(useWorkspace.getState().status).toBe('READY');
  });

  it('clearFile resets to EMPTY', async () => {
    mockProbe.mockResolvedValue({
      id: 'A', path: '/x.mp4', filename: 'x.mp4', sizeBytes: 1,
      container: { format: 'mp4', longName: '' }, audio: null,
      compatibility: { supported: true, issues: [], warnings: [] },
    });
    await useWorkspace.getState().loadFile('/x.mp4');
    useWorkspace.getState().clearFile();
    expect(useWorkspace.getState().status).toBe('EMPTY');
    expect(useWorkspace.getState().result).toBeUndefined();
  });

  it('replaceFile clears then loads the new path', async () => {
    mockProbe.mockResolvedValue({
      id: 'A', path: '/y.mp4', filename: 'y.mp4', sizeBytes: 1,
      container: { format: 'mp4', longName: '' }, audio: null,
      compatibility: { supported: true, issues: [], warnings: [] },
    });
    await useWorkspace.getState().loadFile('/x.mp4');
    await useWorkspace.getState().replaceFile('/y.mp4');
    expect(useWorkspace.getState().status).toBe('READY');
    expect(useWorkspace.getState().result?.path).toBe('/y.mp4');
  });

  it('retry is a no-op when there is no path', async () => {
    await useWorkspace.getState().retry();
    expect(useWorkspace.getState().status).toBe('EMPTY');
  });
});
