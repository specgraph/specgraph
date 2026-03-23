// Shared reactive project state
let current = $state('');
let available = $state<string[]>([]);
let loaded = $state(false);

export const project = {
  get current() { return current; },
  set current(v: string) { current = v; },
  get available() { return available; },
  get loaded() { return loaded; },
};

export async function loadProjects(): Promise<void> {
  try {
    const resp = await fetch('/api/projects');
    if (resp.ok) {
      const data = await resp.json();
      available = data.projects ?? [];
      if (available.length > 0 && !current) {
        current = available[0];
      }
    }
  } catch {
    // Fall back silently
  }
  loaded = true;
}
