// Shared preset metadata. There is no backend /presets endpoint, so this
// registry mirrors processor/preset.go. Preset `name`s must match the backend
// verbatim — they are sent as-is in the upload's `presets` field and arrive
// back on each SSE event's `preset` field.
//
// Consumed by PresetSelector.svelte (the grouped checkbox grid) and
// ProgressCard.svelte (per-preset label + dimensions). Keep one source of truth.

export const PRESET_GROUPS = [
  {
    category: 'Website',
    presets: [
      { name: 'website_webp', label: 'WebP', dims: 'Original size' },
      { name: 'website_avif', label: 'AVIF', dims: 'Original size' },
    ],
  },
  {
    category: 'Instagram',
    presets: [
      { name: 'instagram_square', label: 'Square', dims: '1080×1080' },
      { name: 'instagram_portrait', label: 'Portrait', dims: '1080×1350' },
    ],
  },
  {
    category: 'LinkedIn',
    presets: [{ name: 'linkedin', label: 'Post', dims: '1200×627' }],
  },
  {
    category: 'Twitter / X',
    presets: [{ name: 'twitter', label: 'Post', dims: '1200×675' }],
  },
  {
    category: 'Open Graph',
    presets: [{ name: 'og_image', label: 'OG image', dims: '1200×630' }],
  },
];

export const PRESET_NAMES = PRESET_GROUPS.flatMap((g) =>
  g.presets.map((p) => p.name),
);

// name → { label, category, dims } lookup, for rendering a single preset by name.
export const PRESET_META = Object.fromEntries(
  PRESET_GROUPS.flatMap((g) =>
    g.presets.map((p) => [
      p.name,
      { label: p.label, category: g.category, dims: p.dims },
    ]),
  ),
);
