import { describe, it, expect } from 'vitest';
import { basename } from './path';

describe('basename', () => {
  it('returns the final segment of a POSIX path', () => {
    expect(basename('/Users/me/clips/take 1.mp4')).toBe('take 1.mp4');
  });

  it('returns the final segment of a Windows path', () => {
    expect(basename('C:\\Users\\me\\clips\\take 1.mp4')).toBe('take 1.mp4');
  });

  it('handles mixed separators', () => {
    expect(basename('C:/Users\\me/clip.mov')).toBe('clip.mov');
  });

  it('returns the input when there is no separator', () => {
    expect(basename('clip.mov')).toBe('clip.mov');
  });

  it('returns an empty string for nullish input', () => {
    expect(basename(undefined)).toBe('');
    expect(basename(null)).toBe('');
    expect(basename('')).toBe('');
  });
});
