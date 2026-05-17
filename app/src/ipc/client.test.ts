import { describe, it, expect } from 'vitest';
import { toIpcError } from './client';

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
});
