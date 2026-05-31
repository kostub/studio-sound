import { describe, it, expect, vi } from 'vitest';

// Mock @tauri-apps/api/core before any module that calls invoke is imported.
vi.mock('@tauri-apps/api/core', () => ({
  invoke: vi.fn(),
}));

import { invoke } from '@tauri-apps/api/core';
import { toIpcError, probe } from './client';

const mockInvoke = vi.mocked(invoke);

describe('toIpcError', () => {
  it('returns structured error as-is when shape matches', () => {
    const e = toIpcError({ code: 'FILE_NOT_FOUND', message: 'missing' });
    expect(e).toEqual({ code: 'FILE_NOT_FOUND', message: 'missing' });
  });

  it('preserves details field when present', () => {
    const e = toIpcError({
      code: 'CORRUPT_MEDIA',
      message: 'invalid data',
      details: { stderrTail: 'moov atom not found' },
    });
    expect(e.code).toBe('CORRUPT_MEDIA');
    expect(e.details).toEqual({ stderrTail: 'moov atom not found' });
  });

  it('falls back to UNKNOWN for non-object string rejection', () => {
    const e = toIpcError('boom');
    expect(e.code).toBe('UNKNOWN');
    expect(e.message).toBe('boom');
  });

  it('falls back to UNKNOWN for null/undefined', () => {
    expect(toIpcError(null).code).toBe('UNKNOWN');
    expect(toIpcError(undefined).code).toBe('UNKNOWN');
  });

  it('falls back to UNKNOWN for partial-shape object missing message', () => {
    const e = toIpcError({ code: 'FILE_NOT_FOUND' });
    expect(e.code).toBe('UNKNOWN');
    expect(e.message).toBe('[object Object]');
  });
});

describe('probe', () => {
  it('invokes media_probe with the given path', async () => {
    mockInvoke.mockResolvedValueOnce({
      id: 'A', path: '/x', filename: 'x', sizeBytes: 1,
      container: { format: 'mov,mp4', longName: '' },
      audio: null,
      compatibility: { supported: false, issues: ['no audio'], warnings: [] },
    });
    const r = await probe('/x');
    expect(invoke).toHaveBeenCalledWith('media_probe', { path: '/x' });
    expect(r.id).toBe('A');
    expect(r.path).toBe('/x');
    expect(r.audio).toBeNull();
    expect(r.compatibility.supported).toBe(false);
  });

  it('throws IpcError on structured rejection', async () => {
    mockInvoke.mockRejectedValueOnce({
      code: 'FILE_NOT_FOUND', message: 'missing',
    });
    await expect(probe('/x')).rejects.toMatchObject({ code: 'FILE_NOT_FOUND' });
  });
});
