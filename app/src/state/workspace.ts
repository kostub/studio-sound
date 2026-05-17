import { create } from 'zustand';
import { probe } from '../ipc/client';
import type { ProbeResult } from '../ipc/generated/media.probe';
import type { IpcError } from '../ipc/client';

export type WorkspaceStatus =
  | 'EMPTY'
  | 'FILE_LOADED'
  | 'PROBING'
  | 'READY'
  | 'ERROR'
  | 'RETRYING'
  | 'REMOVED';

// Data fields are stored as `T | undefined` (not optional) so that
// `exactOptionalPropertyTypes` does not prevent explicit `undefined` assignments
// inside Zustand's `set()` updater.
export interface WorkspaceState {
  status: WorkspaceStatus;
  path: string | undefined;
  result: ProbeResult | undefined;
  error: IpcError | undefined;
  loadFile: (path: string) => Promise<void>;
  replaceFile: (path: string) => Promise<void>;
  clearFile: () => void;
  retry: () => Promise<void>;
}

const EMPTY: Pick<WorkspaceState, 'status' | 'path' | 'result' | 'error'> = {
  status: 'EMPTY',
  path: undefined,
  result: undefined,
  error: undefined,
};

export const useWorkspace = create<WorkspaceState>((set, get) => ({
  ...EMPTY,

  async loadFile(path: string) {
    set({ status: 'FILE_LOADED', path, result: undefined, error: undefined });
    set({ status: 'PROBING' });
    try {
      const result = await probe(path);
      set({ status: 'READY', result, error: undefined });
    } catch (e) {
      set({ status: 'ERROR', error: e as IpcError, result: undefined });
    }
  },

  async replaceFile(path: string) {
    set({ status: 'REMOVED', result: undefined, error: undefined });
    await get().loadFile(path);
  },

  clearFile() {
    set({ ...EMPTY });
  },

  async retry() {
    const { path } = get();
    if (!path) return;
    set({ status: 'RETRYING', error: undefined });
    try {
      const result = await probe(path);
      set({ status: 'READY', result, error: undefined });
    } catch (e) {
      set({ status: 'ERROR', error: e as IpcError });
    }
  },
}));

// For tests only — resets the store between cases.
export function resetWorkspaceForTest(): void {
  useWorkspace.setState({ ...EMPTY });
}

// Selector helpers used by components.
export const useWorkspaceStatus = (): WorkspaceStatus => useWorkspace((s) => s.status);
export const useWorkspaceFile = (): {
  path: string | undefined;
  result: ProbeResult | undefined;
  error: IpcError | undefined;
} => useWorkspace((s) => ({ path: s.path, result: s.result, error: s.error }));
