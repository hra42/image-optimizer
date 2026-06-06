// Shared preset metadata. There is no backend /presets endpoint, so this
// registry mirrors processor/preset.go. Preset `name`s must match the backend
// verbatim — they are sent as-is in the upload's `presets` field and arrive
// back on each SSE event's `preset` field.
//
// Consumed by PresetSelector.svelte (the grouped checkbox grid) and
// ProgressCard.svelte (per-preset label + dimensions). Keep one source of truth.

// Each group carries an `accent` — a Catppuccin color token (the bare name, e.g.
// 'mauve') used to tint that category's pills and progress bars. Keep the
// accents distinct between adjacent groups so the grid reads as color-coded.
//
// Each preset also carries `help`: a short, plain-language hint shown in the
// (?) tooltip next to its pill, so a user who doesn't know the formats can pick
// confidently. Keep these one or two sentences — they render in a small popover.
export const PRESET_GROUPS = [
  {
    category: 'Convert (full size)',
    accent: 'mauve',
    presets: [
      { name: 'convert_jpeg', label: 'To JPEG', dims: 'Original size · high quality', help: 'Converts to JPEG at full resolution. Universal — opens anywhere. Good for photos; no transparency.' },
      { name: 'convert_png', label: 'To PNG', dims: 'Original size · lossless', help: 'Converts to PNG at full resolution, lossless. Keeps transparency and sharp edges; larger files. Best for logos, screenshots, graphics.' },
      { name: 'convert_webp', label: 'To WebP', dims: 'Original size · high quality', help: 'Converts to WebP at full resolution. Much smaller than JPEG/PNG with great quality and wide support. A safe modern default.' },
      { name: 'convert_avif', label: 'To AVIF', dims: 'Original size · high quality', help: 'Converts to AVIF at full resolution. The smallest files of all, newest format. Choose for maximum savings if your audience uses modern browsers.' },
    ],
  },
  {
    category: 'Compress',
    accent: 'flamingo',
    presets: [
      { name: 'compress_best', label: 'Best quality', dims: 'Original size · keeps format', help: 'Makes the file smaller while staying near-indistinguishable from the original. Keeps the same format (JPEG stays JPEG, PNG stays PNG, WebP stays WebP). For "smaller but I won\'t notice."' },
      { name: 'compress_balanced', label: 'Balanced', dims: 'Original size · keeps format', help: 'The everyday web default: a strong size reduction with great quality. Keeps the source format and dimensions. Pick this if unsure.' },
      { name: 'compress_max', label: 'Max savings', dims: 'Original size · keeps format', help: 'The smallest file, for email, slow connections, or tight upload limits — quality is reduced more noticeably. Keeps the source format. (PNG savings are modest except here, where it switches to a reduced colour palette.)' },
    ],
  },
  {
    category: 'Website',
    accent: 'blue',
    presets: [
      { name: 'website_webp', label: 'WebP', dims: 'Original size', help: 'Best all-round web format: small files, excellent quality, supported by all modern browsers. If unsure, pick this.' },
      { name: 'website_avif', label: 'AVIF', dims: 'Original size', help: 'Smallest files for the web, newest format. Slightly slower to encode and not supported by a few older browsers. Great paired with a JPEG fallback.' },
      { name: 'jpeg_original', label: 'JPEG', dims: 'Original size', help: 'The universal fallback. Works everywhere, ideal for photographs. No transparency support.' },
      { name: 'png_original', label: 'PNG', dims: 'Original size', help: 'Lossless with transparency. Best for logos, icons, and graphics with sharp edges or text. Larger than WebP.' },
    ],
  },
  {
    category: 'Instagram',
    accent: 'pink',
    presets: [
      { name: 'instagram_square', label: 'Square', dims: '1080×1080', help: 'Classic 1:1 Instagram feed post. Crops/fits your image to a perfect square.' },
      { name: 'instagram_portrait', label: 'Portrait', dims: '1080×1350', help: 'Tall 4:5 Instagram post — takes up more screen space in the feed than a square.' },
      { name: 'instagram_story', label: 'Story', dims: '1080×1920', help: 'Full-screen 9:16 Instagram Story or Reel. Fills the whole phone screen vertically.' },
    ],
  },
  {
    category: 'LinkedIn',
    accent: 'sapphire',
    presets: [
      { name: 'linkedin', label: 'Post', dims: '1200×627', help: 'Recommended size for a LinkedIn shared post or link preview image.' },
      { name: 'linkedin_profile_banner', label: 'Profile banner', dims: '1584×396', help: 'Cover/background image for a personal LinkedIn profile.' },
      { name: 'linkedin_company_banner', label: 'Company banner', dims: '1128×191', help: 'Cover image for a LinkedIn company / organization page.' },
    ],
  },
  {
    category: 'Twitter / X',
    accent: 'sky',
    presets: [{ name: 'twitter', label: 'Post', dims: '1200×675', help: '16:9 image for an X / Twitter post or link card — fills the timeline preview without cropping.' }],
  },
  {
    category: 'Facebook',
    accent: 'lavender',
    presets: [{ name: 'facebook_post', label: 'Post', dims: '1200×630', help: 'Standard Facebook shared-post and link-preview size.' }],
  },
  {
    category: 'Pinterest',
    accent: 'red',
    presets: [{ name: 'pinterest_pin', label: 'Pin', dims: '1000×1500', help: 'Tall 2:3 Pinterest Pin — the vertical shape Pinterest recommends for best reach.' }],
  },
  {
    category: 'Open Graph',
    accent: 'teal',
    presets: [{ name: 'og_image', label: 'OG image', dims: '1200×630', help: 'The preview image shown when your page is shared on social apps and chat (the og:image). Use this if you’re setting up link previews for a website.' }],
  },
  {
    // Documents are BUNDLE presets: every uploaded image becomes one page of a
    // single multi-page PDF (in upload order), instead of one output per image.
    // Mirrors KindDocumentPDF in processor/preset.go; names listed in
    // BUNDLE_PRESETS below.
    category: 'Documents',
    accent: 'green',
    presets: [
      { name: 'linkedin_doc_portrait', label: 'LinkedIn doc — portrait', dims: '1080×1350 · multi-page PDF', help: 'Combines ALL uploaded images into one multi-page PDF — a LinkedIn document / carousel post — in upload order. Each image becomes a portrait page.' },
      { name: 'linkedin_doc_square', label: 'LinkedIn doc — square', dims: '1080×1080 · multi-page PDF', help: 'Combines ALL uploaded images into one multi-page PDF, in upload order. Each image becomes a square page.' },
    ],
  },
  {
    category: 'Icons & Thumbnails',
    accent: 'yellow',
    presets: [
      { name: 'favicon', label: 'Favicon pack', dims: 'Full icon set + manifest', help: 'A complete drop-in icon set for a website: favicon, apple-touch-icon, PWA icons, and a manifest. Use a square source image for best results.' },
      { name: 'thumbnail', label: 'Thumbnail', dims: '400×400', help: 'A small 400×400 preview image — handy for galleries, lists, or avatars.' },
    ],
  },
  {
    category: 'Banners',
    accent: 'peach',
    presets: [
      { name: 'email_header', label: 'Email header', dims: '600×200', help: 'Wide 600×200 banner sized for the header of an email newsletter.' },
      { name: 'web_banner', label: 'Web banner', dims: '1920×480', help: 'Full-width 1920×480 hero/banner strip for the top of a web page.' },
    ],
  },
];

export const PRESET_NAMES = PRESET_GROUPS.flatMap((g) =>
  g.presets.map((p) => p.name),
);

// BUNDLE_PRESETS are the presets that consume ALL uploaded files at once and
// emit a single combined output (a multi-page PDF) rather than one output per
// image. Mirror of Preset.IsBundle() / KindDocumentPDF in processor/preset.go —
// keep the two in sync. Used so progress accounting expects ONE unit for these
// (not one per file). See stores/progress.svelte.js.
export const BUNDLE_PRESETS = new Set([
  'linkedin_doc_portrait',
  'linkedin_doc_square',
]);

// name → { label, category, dims, accent, help } lookup, for rendering a single
// preset by name (its label, dimensions, the category's accent color, and the
// help hint).
export const PRESET_META = Object.fromEntries(
  PRESET_GROUPS.flatMap((g) =>
    g.presets.map((p) => [
      p.name,
      { label: p.label, category: g.category, dims: p.dims, accent: g.accent, help: p.help },
    ]),
  ),
);
