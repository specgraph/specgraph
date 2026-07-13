// Categorical badge palette — the ONE color carve-out (D-10, UI-SPEC).
//
// These are CATEGORICAL DATA ENCODINGS, not part of the 60/30/10 theme accent
// budget. They intentionally do NOT map to `--primary`/theme tokens: each
// category keeps a fixed, colorblind-distinguishable hue via explicit
// light/dark Tailwind utility class pairs so it stays stable across themes and
// project switches. Consume these maps from `Badge class={...}` in the
// constitution, spec-detail, decision-detail, SpecTable, and FindingsSection
// views. Merged/Layer view badge uses the neutral shadcn `Badge
// variant="secondary"` instead (not defined here).

/** A fixed light/dark Tailwind class pair for a categorical badge. */
export type CategoricalBadgeClass = string;

const NEUTRAL: CategoricalBadgeClass =
	"bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";

/**
 * Constitution layers (D-10) — pinned by UI-SPEC "Categorical badge palette".
 * Keys match `layerOf()` output in constitution/+page.svelte (lowercase).
 */
export const layerBadgeVariants = {
	user: "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300",
	org: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
	project: "bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300",
	domain: "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-300",
} as const satisfies Record<string, CategoricalBadgeClass>;

export type ConstitutionLayerKey = keyof typeof layerBadgeVariants;

/**
 * Spec lifecycle stage badges. Keys match `spec.stage` strings used by
 * SpecTable.svelte / spec detail.
 */
export const stageBadgeVariants = {
	spark: "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-300",
	shape: "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300",
	specify: "bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300",
	decompose: "bg-yellow-100 text-yellow-800 dark:bg-yellow-950 dark:text-yellow-300",
	approved: "bg-teal-100 text-teal-800 dark:bg-teal-950 dark:text-teal-300",
	in_progress: "bg-orange-100 text-orange-800 dark:bg-orange-950 dark:text-orange-300",
	done: NEUTRAL,
} as const satisfies Record<string, CategoricalBadgeClass>;

export type SpecStageKey = keyof typeof stageBadgeVariants;

/**
 * Decision-status badges. Keys match `statusLabel()` output in
 * decision/[...slug]/+page.svelte.
 */
export const statusBadgeVariants = {
	proposed: "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-300",
	accepted: "bg-teal-100 text-teal-800 dark:bg-teal-950 dark:text-teal-300",
	deprecated: NEUTRAL,
	superseded: "bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400",
} as const satisfies Record<string, CategoricalBadgeClass>;

export type DecisionStatusKey = keyof typeof statusBadgeVariants;

/**
 * Finding-severity badges. Keys match `severityClass()` output in
 * FindingsSection.svelte (info | warning | error).
 */
export const severityBadgeVariants = {
	info: "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300",
	warning: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300",
	error: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300",
} as const satisfies Record<string, CategoricalBadgeClass>;

export type FindingSeverityKey = keyof typeof severityBadgeVariants;

/** Lookup helper that falls back to the neutral pair for unknown keys. */
function pick<T extends Record<string, CategoricalBadgeClass>>(
	map: T,
	key: string | undefined | null,
): CategoricalBadgeClass {
	return (key != null && map[key as keyof T]) || NEUTRAL;
}

export const layerBadgeClass = (key: string | null | undefined) =>
	pick(layerBadgeVariants, key);
export const stageBadgeClass = (key: string | null | undefined) =>
	pick(stageBadgeVariants, key);
export const statusBadgeClass = (key: string | null | undefined) =>
	pick(statusBadgeVariants, key);
export const severityBadgeClass = (key: string | null | undefined) =>
	pick(severityBadgeVariants, key);
