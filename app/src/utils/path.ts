/**
 * Extract the final segment of a filesystem path, handling both POSIX (`/`) and
 * Windows (`\`) separators. Returns an empty string for nullish input.
 */
export function basename(path?: string | null): string {
  if (!path) return '';
  const m = path.match(/[^/\\]+$/);
  return m ? m[0] : path;
}
