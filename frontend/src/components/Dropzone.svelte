<script>
  // Dropzone: native drag-and-drop with a click-to-browse fallback. The
  // selected files are bound back to the parent via `files`. Touch devices
  // (which don't support drag-and-drop) use the click fallback.

  import CropModal from './CropModal.svelte';
  import { cropAspects } from '../lib/presets.js';

  let { files = $bindable([]), selectedPresets = [], disabled = false } = $props();

  // The file entry whose crop is being adjusted (drives CropModal), or null.
  let cropping = $state(null);

  // True when at least one selected preset crops to a fixed shape, so the
  // "Adjust crop" affordance is meaningful. When false the button is hidden to
  // keep the default flow clean (the focal point would have no effect).
  const anyCrops = $derived(selectedPresets.some((n) => n in cropAspects));

  let dragActive = $state(false);
  let inputEl = $state(null);
  // Drag events fire on child elements too; count enter/leave so the highlight
  // only clears when the cursor truly leaves the zone.
  let dragDepth = 0;

  // Accepted input types, mirroring the backend (handlers/upload.go).
  // HEIC/HEIF are iPhone photos — libvips decodes them server-side.
  const ACCEPT_EXT = ['.jpg', '.jpeg', '.png', '.webp', '.avif', '.heic', '.heif', '.svg'];
  const ACCEPT_MIME = [
    'image/jpeg',
    'image/png',
    'image/webp',
    'image/avif',
    'image/heic',
    'image/heif',
    'image/svg+xml',
  ];

  function isAccepted(file) {
    const name = file.name.toLowerCase();
    const byExt = ACCEPT_EXT.some((ext) => name.endsWith(ext));
    // AVIF and HEIC MIME are unreliable across browsers, so accept on
    // extension too.
    const byMime = ACCEPT_MIME.includes(file.type);
    return byExt || byMime;
  }

  function fileKey(file) {
    return `${file.name}:${file.size}`;
  }

  function addFiles(fileList) {
    const incoming = Array.from(fileList).filter(isAccepted);
    if (incoming.length === 0) return;
    const seen = new Set(files.map(fileKey));
    const additions = [];
    for (const f of incoming) {
      const key = fileKey(f);
      if (seen.has(key)) continue;
      seen.add(key);
      additions.push({ file: f, url: URL.createObjectURL(f) });
    }
    if (additions.length > 0) {
      files = [...files, ...additions];
    }
  }

  // True when the selection includes any SVG. SVG is vector, so the full-size
  // presets (Convert / Website) rasterize at a fixed density rather than a
  // pixel size copied from the source — worth flagging so the output dimensions
  // aren't a surprise. Mirrors the density bump in processor/pipeline.go.
  const hasSVG = $derived(
    files.some((entry) => entry.file.name.toLowerCase().endsWith('.svg')),
  );

  function removeAt(i) {
    const entry = files[i];
    if (entry) URL.revokeObjectURL(entry.url);
    files = files.filter((_, idx) => idx !== i);
  }

  // Revoke any outstanding object URLs when the component is torn down.
  $effect(() => {
    return () => {
      for (const entry of files) URL.revokeObjectURL(entry.url);
    };
  });

  function onDrop(e) {
    e.preventDefault();
    dragActive = false;
    dragDepth = 0;
    if (disabled) return;
    if (e.dataTransfer?.files?.length) addFiles(e.dataTransfer.files);
  }

  function onDragOver(e) {
    e.preventDefault();
  }

  function onDragEnter(e) {
    e.preventDefault();
    if (disabled) return;
    dragDepth += 1;
    dragActive = true;
  }

  function onDragLeave() {
    dragDepth = Math.max(0, dragDepth - 1);
    if (dragDepth === 0) dragActive = false;
  }

  function onInputChange(e) {
    if (e.target.files?.length) addFiles(e.target.files);
    // Reset so re-selecting the same file fires change again.
    e.target.value = '';
  }

  function openPicker() {
    if (!disabled) inputEl?.click();
  }

  function formatSize(bytes) {
    if (bytes < 1024) return `${bytes} B`;
    const units = ['KB', 'MB', 'GB'];
    let size = bytes / 1024;
    let i = 0;
    while (size >= 1024 && i < units.length - 1) {
      size /= 1024;
      i += 1;
    }
    return `${size.toFixed(size < 10 ? 1 : 0)} ${units[i]}`;
  }
</script>

<div class="flex flex-col gap-4">
  <button
    type="button"
    class="group flex w-full flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-6 py-12 text-center transition-all duration-300
           {dragActive
      ? 'scale-[1.01] border-ctp-mauve bg-gradient-to-br from-ctp-surface0 to-ctp-base shadow-lg shadow-ctp-mauve/30'
      : 'border-ctp-surface1 bg-ctp-base hover:border-ctp-mauve/60 hover:bg-ctp-surface0/40'}
           {disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'}"
    {disabled}
    ondrop={onDrop}
    ondragover={onDragOver}
    ondragenter={onDragEnter}
    ondragleave={onDragLeave}
    onclick={openPicker}
  >
    <svg
      class="h-10 w-10 transition-all duration-300 {dragActive
        ? '-translate-y-1 scale-110 text-ctp-mauve'
        : 'text-ctp-overlay0 group-hover:-translate-y-0.5 group-hover:text-ctp-mauve'}"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="1.5"
      aria-hidden="true"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5"
      />
    </svg>
    <span class="text-lg font-medium {dragActive ? 'text-ctp-mauve' : 'text-ctp-text'}">
      {dragActive ? 'Drop images to add them' : 'Drag & drop images here'}
    </span>
    <span class="text-base text-ctp-subtext1">
      or click to browse — JPEG, PNG, WebP, AVIF, HEIC, SVG
    </span>
  </button>

  <input
    bind:this={inputEl}
    type="file"
    class="hidden"
    multiple
    accept="image/jpeg,image/png,image/webp,image/avif,image/heic,image/heif,image/svg+xml,.jpg,.jpeg,.png,.webp,.avif,.heic,.heif,.svg"
    onchange={onInputChange}
  />

  {#if files.length > 0}
    <ul class="grid grid-cols-2 gap-3 sm:grid-cols-3">
      {#each files as entry, i (entry.url)}
        <li
          class="animate-fade-up relative flex items-center gap-3 rounded-lg border border-ctp-surface1 bg-ctp-surface0 p-2 transition-all duration-200 hover:-translate-y-0.5 hover:border-ctp-mauve/50 hover:shadow-md hover:shadow-ctp-mauve/10"
          style="animation-delay: {Math.min(i, 12) * 40}ms"
        >
          <img
            src={entry.url}
            alt={entry.file.name}
            class="h-12 w-12 flex-none rounded object-cover ring-1 ring-ctp-surface1"
          />
          <div class="min-w-0 flex-1">
            <p class="truncate text-base text-ctp-text" title={entry.file.name}>
              {entry.file.name}
            </p>
            <p class="text-sm text-ctp-subtext1">{formatSize(entry.file.size)}</p>
            {#if anyCrops}
              <button
                type="button"
                class="mt-1 inline-flex items-center gap-1 text-xs font-medium transition-colors {entry.focal
                  ? 'text-ctp-mauve hover:text-ctp-lavender'
                  : 'text-ctp-blue hover:text-ctp-mauve'}"
                onclick={() => (cropping = entry)}
              >
                <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 5h10a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2Z" />
                </svg>
                {entry.focal ? 'Crop adjusted' : 'Adjust crop'}
              </button>
            {/if}
          </div>
          <button
            type="button"
            class="flex-none rounded p-1 text-ctp-overlay0 transition-all duration-200 hover:rotate-90 hover:scale-110 hover:bg-ctp-red/10 hover:text-ctp-red"
            aria-label="Remove {entry.file.name}"
            onclick={() => removeAt(i)}
          >
            <svg
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
              aria-hidden="true"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
            </svg>
          </button>
        </li>
      {/each}
    </ul>
  {/if}

  {#if hasSVG}
    <p
      class="animate-fade-up flex items-start gap-2 rounded-lg border border-ctp-mauve/30 bg-ctp-mauve/5 px-3 py-2 text-sm leading-relaxed text-ctp-subtext1"
    >
      <svg
        class="mt-0.5 h-4 w-4 flex-none text-ctp-mauve"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2"
        aria-hidden="true"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M11.25 11.25l.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z"
        />
      </svg>
      <span>
        <span class="font-semibold text-ctp-text">SVG is vector.</span>
        Social and icon presets render it sharp at their exact size. The full-size
        <span class="font-medium text-ctp-mauve">Convert</span> / <span class="font-medium text-ctp-blue">Website</span>
        presets rasterize at 4× the SVG’s drawing size (e.g. a 256px icon → ~1024px).
      </span>
    </p>
  {/if}
</div>

<CropModal bind:open={cropping} {selectedPresets} />
