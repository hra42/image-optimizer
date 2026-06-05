<script>
  // Dropzone: native drag-and-drop with a click-to-browse fallback. The
  // selected files are bound back to the parent via `files`. Touch devices
  // (which don't support drag-and-drop) use the click fallback.

  let { files = $bindable([]), disabled = false } = $props();

  let dragActive = $state(false);
  let inputEl = $state(null);
  // Drag events fire on child elements too; count enter/leave so the highlight
  // only clears when the cursor truly leaves the zone.
  let dragDepth = 0;

  // Accepted input types, mirroring the backend (handlers/upload.go).
  // HEIC/HEIF are iPhone photos — libvips decodes them server-side.
  const ACCEPT_EXT = ['.jpg', '.jpeg', '.png', '.webp', '.avif', '.heic', '.heif'];
  const ACCEPT_MIME = [
    'image/jpeg',
    'image/png',
    'image/webp',
    'image/avif',
    'image/heic',
    'image/heif',
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
    class="flex w-full flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-6 py-12 text-center transition-colors
           {dragActive
      ? 'border-ctp-mauve bg-ctp-surface0'
      : 'border-ctp-surface1 bg-ctp-base hover:border-ctp-overlay0'}
           {disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'}"
    {disabled}
    ondrop={onDrop}
    ondragover={onDragOver}
    ondragenter={onDragEnter}
    ondragleave={onDragLeave}
    onclick={openPicker}
  >
    <svg
      class="h-10 w-10 {dragActive ? 'text-ctp-mauve' : 'text-ctp-overlay0'}"
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
    <span class="text-lg font-medium text-ctp-text">
      {dragActive ? 'Drop images to add them' : 'Drag & drop images here'}
    </span>
    <span class="text-base text-ctp-subtext1">
      or click to browse — JPEG, PNG, WebP, AVIF, HEIC
    </span>
  </button>

  <input
    bind:this={inputEl}
    type="file"
    class="hidden"
    multiple
    accept="image/jpeg,image/png,image/webp,image/avif,image/heic,image/heif,.jpg,.jpeg,.png,.webp,.avif,.heic,.heif"
    onchange={onInputChange}
  />

  {#if files.length > 0}
    <ul class="grid grid-cols-2 gap-3 sm:grid-cols-3">
      {#each files as entry, i (entry.url)}
        <li
          class="relative flex items-center gap-3 rounded-lg border border-ctp-surface1 bg-ctp-surface0 p-2"
        >
          <img
            src={entry.url}
            alt={entry.file.name}
            class="h-12 w-12 flex-none rounded object-cover"
          />
          <div class="min-w-0 flex-1">
            <p class="truncate text-base text-ctp-text" title={entry.file.name}>
              {entry.file.name}
            </p>
            <p class="text-sm text-ctp-subtext1">{formatSize(entry.file.size)}</p>
          </div>
          <button
            type="button"
            class="flex-none rounded p-1 text-ctp-overlay0 hover:text-ctp-red"
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
</div>
