// Shared reactive project state, persisted in localStorage.
const STORAGE_KEY = 'specgraph-project';

let current = $state(
  typeof localStorage !== 'undefined' ? localStorage.getItem(STORAGE_KEY) ?? '' : ''
);
let available = $state<string[]>([]);
let loaded = $state(false);

export const project = {
  get current() { return current; },
  set current(v: string) {
    current = v;
    if (typeof localStorage !== 'undefined') localStorage.setItem(STORAGE_KEY, v);
  },
  get available() { return available; },
  get loaded() { return loaded; },
};

export async function loadProjects(): Promise<void> {
  try {
    const resp = await fetch('/api/projects');
    if (resp.ok) {
      const data = await resp.json();
      available = data.projects ?? [];
      // Use saved project if it's still valid, otherwise pick the first
      if (current && available.includes(current)) {
        // keep it
      } else if (available.length > 0) {
        project.current = available[0]; // triggers localStorage save
      }
    }
  } catch {
    // Fall back silently
  }
  loaded = true;
}
