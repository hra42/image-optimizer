<script>
  // BgRemoveModal: opt-in, in-browser background removal for one file entry.
  // The model runs in a Web Worker (lib/bgremove.js); the image never leaves the
  // browser for this step. On apply, the entry's file is swapped for the
  // transparent PNG cutout (the original is kept on `entry.original` so it can be
  // restored), and `entry.matte` records the background color the server uses
  // when flattening for formats without alpha (JPEG).
  //
  // Controlled by the parent via `open` (the entry, or null), like CropModal.

  import { untrack } from 'svelte';
  import { BG_MODELS } from '../lib/bgModels.js';
  import { isModelCached, pickModel, removeBackground } from '../lib/bgremove.js';

  let { open = $bindable(null), baseUrl = '', maxFileBytes = 0 } = $props();

  // idle → (download/load/infer) → done | error
  let phase = $state('idle');
  let progress = $state(0); // download fraction 0..1
  let modelKey = $state(null);
  let cached = $state(false);
  let error = $state('');
  let result = $state(null); // { blob, url }
  // The cutout object URL this modal still owns (plain variable, not state:
  // ownership must not depend on effect timing). apply() hands it to the entry.
  let ownedUrl = null;
  // Background for formats without transparency: null = transparent (server
  // defaults JPEG to white), or '#rrggbb'.
  let matte = $state(null);
  let customColor = $state('#ffffff');
  let runToken = 0;

  const MATTES = [
    { value: null, label: 'Transparent' },
    { value: '#ffffff', label: 'White' },
    { value: '#000000', label: 'Black' },
  ];

  // Reset per opened entry; probe which model this device will use and whether
  // it still needs downloading, so the first-run notice can show the size.
  $effect(() => {
    const entry = open;
    if (!entry) return;
    untrack(() => {
      phase = 'idle';
      error = '';
      progress = 0;
      matte = entry.matte ?? null;
      if (matte && !MATTES.some((m) => m.value === matte)) customColor = matte;
      pickModel().then(async (key) => {
        if (open !== entry) return;
        modelKey = key;
        cached = await isModelCached(baseUrl, key);
      });
    });
    return () => {
      runToken++;
      discardResult();
    };
  });

  const model = $derived(modelKey ? BG_MODELS[modelKey] : null);
  const busy = $derived(phase === 'download' || phase === 'load' || phase === 'infer');
  const tooBig = $derived(!!result && maxFileBytes > 0 && result.blob.size > maxFileBytes);

  function discardResult() {
    if (ownedUrl) URL.revokeObjectURL(ownedUrl);
    ownedUrl = null;
    result = null;
  }

  async function start() {
    const token = ++runToken;
    discardResult();
    error = '';
    phase = cached ? 'load' : 'download';
    const source = open.original?.file ?? open.file;
    try {
      const out = await removeBackground(source, baseUrl, (p) => {
        if (token !== runToken) return;
        phase = p.phase;
        if (p.phase === 'download') progress = p.loaded / p.total;
      });
      if (token !== runToken) return;
      modelKey = out.model;
      cached = true;
      ownedUrl = URL.createObjectURL(out.blob);
      result = { blob: out.blob, url: ownedUrl };
      phase = 'done';
    } catch (err) {
      if (token !== runToken) return;
      error = err instanceof Error ? err.message : String(err);
      phase = 'error';
    }
  }

  function close() {
    open = null;
  }

  function onKeydown(e) {
    if (open && e.key === 'Escape' && !busy) close();
  }

  function apply() {
    if (!open || !result || tooBig) return;
    const entry = open;
    if (!entry.original) {
      entry.original = { file: entry.file, url: entry.url };
    } else {
      revokeLater(entry.url); // previous cutout
    }
    const base = entry.original.file.name.replace(/\.[^.]+$/, '');
    entry.file = new File([result.blob], `${base}-nobg.png`, { type: 'image/png' });
    entry.url = result.url;
    entry.matte = matte;
    ownedUrl = null; // ownership of the object URL moved to the entry
    result = null;
    close();
  }

  // Change only the background of an entry that already has a cutout.
  function saveMatte() {
    if (open?.original) open.matte = matte;
    close();
  }

  function restore() {
    const entry = open;
    if (!entry?.original) return;
    revokeLater(entry.url);
    entry.file = entry.original.file;
    entry.url = entry.original.url;
    entry.original = null;
    entry.matte = null;
    close();
  }

  // Revoke after the DOM has swapped to the new URL; revoking synchronously
  // makes the still-mounted thumbnail refetch a dead blob URL.
  function revokeLater(url) {
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  function formatMB(bytes) {
    return `${(bytes / (1 << 20)).toFixed(bytes < 10 << 20 ? 1 : 0)} MB`;
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
  <div
    class="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-0 sm:items-center sm:p-4"
    role="dialog"
    aria-modal="true"
    aria-label="Remove background"
    onclick={() => !busy && close()}
  >
    <div
      class="flex max-h-[90vh] w-full max-w-2xl flex-col rounded-t-2xl border border-ctp-surface1 bg-ctp-base shadow-xl sm:rounded-2xl"
      onclick={(e) => e.stopPropagation()}
    >
      <header class="flex items-center justify-between border-b border-ctp-surface1 px-6 py-4">
        <h2 class="text-lg font-bold text-ctp-mauve">Remove background</h2>
        <button
          type="button"
          class="rounded p-1 text-ctp-overlay0 transition-colors hover:text-ctp-text disabled:opacity-40"
          aria-label="Close"
          disabled={busy}
          onclick={close}
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
          </svg>
        </button>
      </header>

      <div class="overflow-y-auto px-6 py-5">
        <p class="mb-4 text-sm leading-relaxed text-ctp-subtext1">
          Cuts the subject out of
          <span class="font-medium text-ctp-text">{(open.original?.file ?? open.file).name}</span>
          right here in your browser — this step doesn't upload the image anywhere.
          {#if model && !cached}
            The first run downloads the {model.label} model
            (<span class="font-medium text-ctp-text">{formatMB(model.bytes)}</span>, cached afterwards).
          {/if}
          {#if modelKey === 'ormbg'}
            <span class="text-ctp-overlay0">
              Your browser has no WebGPU, so a CPU model is used instead: it takes a few
              seconds and fine detail like hair may be less precise.
            </span>
          {/if}
        </p>

        <!-- Preview: the cutout over the chosen matte (or a checkerboard). -->
        <div
          class="relative mx-auto flex w-full items-center justify-center overflow-hidden rounded-lg ring-1 ring-ctp-surface1"
          style={matte
            ? `background:${matte}`
            : 'background: repeating-conic-gradient(#8883 0% 25%, transparent 0% 50%) 50% / 20px 20px'}
        >
          <img
            src={result?.url ?? open.url}
            alt={open.file.name}
            draggable="false"
            class="max-h-[50vh] w-auto transition-opacity {busy ? 'opacity-40' : ''}"
          />
          {#if busy}
            <div class="absolute inset-x-6 bottom-6 flex flex-col gap-2 rounded-lg bg-ctp-base/90 p-3 text-sm text-ctp-text shadow">
              {#if phase === 'download'}
                <span>Downloading model… {Math.round(progress * 100)}%</span>
                <div class="h-1.5 overflow-hidden rounded bg-ctp-surface1">
                  <div class="h-full bg-ctp-mauve transition-all" style="width:{progress * 100}%"></div>
                </div>
              {:else if phase === 'load'}
                <span>Preparing model…</span>
              {:else}
                <span>Removing background…</span>
              {/if}
            </div>
          {/if}
        </div>

        {#if phase === 'error'}
          <p class="mt-3 text-sm text-ctp-red">Background removal failed: {error}</p>
        {/if}
        {#if tooBig}
          <p class="mt-3 text-sm text-ctp-red">
            The cutout is {formatMB(result.blob.size)}, over the {formatMB(maxFileBytes)} upload
            limit. Try a smaller image.
          </p>
        {/if}

        <fieldset class="mt-4 flex flex-wrap items-center gap-2 text-sm">
          <legend class="mb-2 text-ctp-subtext1">
            Background for formats without transparency (JPEG):
          </legend>
          {#each MATTES as m (m.label)}
            <button
              type="button"
              class="rounded-md border px-3 py-1.5 transition-colors {matte === m.value
                ? 'border-ctp-mauve text-ctp-mauve'
                : 'border-ctp-surface1 text-ctp-subtext1 hover:text-ctp-text'}"
              onclick={() => (matte = m.value)}
            >
              {m.label}
            </button>
          {/each}
          <label
            class="inline-flex cursor-pointer items-center gap-2 rounded-md border px-3 py-1.5 transition-colors {matte &&
            !MATTES.some((m) => m.value === matte)
              ? 'border-ctp-mauve text-ctp-mauve'
              : 'border-ctp-surface1 text-ctp-subtext1 hover:text-ctp-text'}"
          >
            Custom
            <input
              type="color"
              class="h-5 w-7 cursor-pointer border-0 bg-transparent p-0"
              bind:value={customColor}
              oninput={() => (matte = customColor)}
            />
          </label>
        </fieldset>
        <p class="mt-2 text-xs text-ctp-overlay0">
          PNG, WebP, AVIF and the favicon pack keep the transparency.
          {#if matte === null}JPEG outputs get a white background.{/if}
        </p>
      </div>

      <footer class="flex items-center justify-between gap-3 border-t border-ctp-surface1 px-6 py-4">
        <button
          type="button"
          class="text-sm text-ctp-subtext1 transition-colors hover:text-ctp-text disabled:opacity-40"
          disabled={!open.original || busy}
          onclick={restore}
        >
          Restore original
        </button>
        {#if phase === 'done'}
          <button
            type="button"
            class="rounded-lg bg-gradient-to-r from-ctp-mauve to-ctp-lavender px-4 py-2 text-sm font-semibold text-ctp-base shadow-md shadow-ctp-mauve/20 transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-ctp-mauve/40 disabled:opacity-40"
            disabled={tooBig}
            onclick={apply}
          >
            Use cutout
          </button>
        {:else}
          <div class="flex items-center gap-3">
          {#if open.original && phase === 'idle'}
            <button
              type="button"
              class="text-sm text-ctp-blue transition-colors hover:text-ctp-mauve"
              onclick={saveMatte}
            >
              Save background
            </button>
          {/if}
          <button
            type="button"
            class="rounded-lg bg-gradient-to-r from-ctp-mauve to-ctp-lavender px-4 py-2 text-sm font-semibold text-ctp-base shadow-md shadow-ctp-mauve/20 transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-ctp-mauve/40 disabled:opacity-40"
            disabled={busy || !model}
            onclick={start}
          >
            {phase === 'error' ? 'Try again' : open.original ? 'Run again' : 'Remove background'}
          </button>
          </div>
        {/if}
      </footer>
    </div>
  </div>
{/if}
