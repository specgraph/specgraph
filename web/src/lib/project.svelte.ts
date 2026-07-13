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
      // D-05: deterministic case-insensitive sort — single client-side source
      // of truth; do not rely on server ordering.
      available = (data.projects ?? [])
        .slice()
        .sort((a: string, b: string) => a.toLowerCase().localeCompare(b.toLowerCase()));
      // D-04 precedence: (1) keep a valid saved project (D-06 falls out of the
      // "in available" guard), (2) prefer one literally named 'default',
      // (3) else the alphabetically-first, (4) else empty (D-07 zero-projects).
      if (current && available.includes(current)) {
        // tier 1: keep the valid saved project
      } else if (available.includes('default')) {
        project.current = 'default'; // tier 2
      } else if (available.length > 0) {
        project.current = available[0]; // tier 3: alpha-first after sort
      } else {
        project.current = ''; // tier 4: no projects available
      }
    }
  } catch {
    // Fall back silently
  }
  loaded = true;
}
