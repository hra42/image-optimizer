<script>
  // App orchestrates the upload flow: idle → uploading → processing → done.
  // It owns the form (files + selected presets) and the upload POST; the
  // progress store (stores/progress.svelte.js) owns the SSE stream and the
  // per-preset progress state that ProgressCard renders.

  import Dropzone from './components/Dropzone.svelte';
  import PresetSelector from './components/PresetSelector.svelte';
  import ProgressCard from './components/ProgressCard.svelte';
  import HowItWorks from './components/HowItWorks.svelte';
  import LegalModal from './components/LegalModal.svelte';
  import CropModal from './components/CropModal.svelte';
  import { createProgress } from './stores/progress.svelte.js';
  import { BUNDLE_PRESETS, PACK_PRESETS } from './lib/presets.js';

  const progress = createProgress();

  let files = $state([]); // [{ file, url }]
  let selectedPresets = $state([]); // preset name strings
  let legalOpen = $state(null); // 'imprint' | 'privacy' | null
  // The file entry whose crop is being adjusted, or null. Owned here (not in
  // Dropzone) so CropModal renders at the page root, escaping the animate-fade-up
  // transform that would otherwise trap its position:fixed overlay.
  let cropping = $state(null);

  let status = $derived(progress.status);
  let canSubmit = $derived(
    status === 'idle' && files.length > 0 && selectedPresets.length > 0,
  );
  let busy = $derived(status === 'uploading' || status === 'processing');

  // True when the job will yield exactly one output file, so the server returns a
  // raw image instead of a ZIP. Mirrors the backend's soleImageOutput rule: one
  // file, one plain image preset (not a multi-file pack, not an all-files bundle).
  let singleImageResult = $derived.by(() => {
    if (files.length !== 1 || selectedPresets.length !== 1) return false;
    const only = selectedPresets[0];
    return !PACK_PRESETS.has(only) && !BUNDLE_PRESETS.has(only);
  });

  async function submit() {
    if (!canSubmit) return;

    progress.start(selectedPresets, files.length, BUNDLE_PRESETS);

    const form = new FormData();
    // Per-file focal points, indexed to the files appended below (same order).
    // null for any file the user didn't adjust → backend keeps its attention
    // crop. Harmless for non-cropping presets, which ignore the focal.
    const focals = files.map((entry) => entry.focal ?? null);
    for (const entry of files) form.append('files', entry.file);
    form.append('focals', JSON.stringify(focals));
    for (const name of selectedPresets) form.append('presets', name);

    let jobId;
    try {
      const res = await fetch('/upload', { method: 'POST', body: form });
      if (!res.ok) {
        progress.fail((await res.text()) || `Upload failed (${res.status})`);
        return;
      }
      ({ jobId } = await res.json());
    } catch (err) {
      progress.fail(err instanceof Error ? err.message : 'Upload failed');
      return;
    }

    progress.connect(jobId);
  }

  function reset() {
    progress.reset();
    files = [];
    selectedPresets = [];
    cropping = null;
    autoDownloaded = false;
  }

  // Triggers a browser download of the given URL without navigating away. Using a
  // transient <a download> click keeps us on the page (so the "Start over" UI and
  // the fallback button remain) while the file saves.
  function triggerDownload(url) {
    const a = document.createElement('a');
    a.href = url;
    a.download = '';
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  // Auto-download once the job completes, so the user gets their file without a
  // click. The manual button stays as a fallback; on the server the first
  // download keeps the job alive and the second (a fallback click) frees it.
  let autoDownloaded = $state(false);
  $effect(() => {
    if (status === 'done' && progress.downloadUrl && !autoDownloaded) {
      autoDownloaded = true;
      triggerDownload(progress.downloadUrl);
    }
  });

  // Ensure the stream is torn down if the component unmounts mid-flight.
  $effect(() => () => progress.close());
</script>

<main class="mx-auto flex min-h-screen w-full max-w-7xl flex-col gap-8 px-4 py-8 sm:px-6 lg:px-10 lg:py-12">
  <header class="flex flex-col gap-1 animate-fade-up">
    <div class="flex items-center gap-3">
      <img
        src="/favicon.svg"
        alt=""
        width="44"
        height="44"
        class="h-10 w-10 flex-none rounded-xl shadow-md shadow-ctp-mauve/20 transition-transform duration-300 hover:scale-105 sm:h-11 sm:w-11"
      />
      <h1 class="text-gradient-aurora text-3xl font-extrabold tracking-tight sm:text-4xl">
        Image Optimizer
      </h1>
    </div>
    <p class="text-sm text-ctp-subtext1 sm:text-base">
      Drop images, pick targets, download the optimized result (a ZIP, or a single
      image when that’s all you asked for).
    </p>
  </header>

  {#if status === 'idle'}
    <!--
      Single full-width column in task order: how-it-works → drop → presets →
      optimize. The inner components (the how-it-works steps and the preset
      grid) spread horizontally to fill the width, so the page uses the whole
      container on desktop without a lopsided multi-column split.
    -->
    <div class="animate-fade-up" style="animation-delay: 60ms">
      <HowItWorks />
    </div>
    <div class="animate-fade-up" style="animation-delay: 120ms">
      <Dropzone bind:files {selectedPresets} bind:cropping />
    </div>
    <div class="animate-fade-up" style="animation-delay: 180ms">
      <PresetSelector bind:selected={selectedPresets} />
    </div>

    <button
      type="button"
      class="group relative overflow-hidden rounded-lg bg-gradient-to-r from-ctp-mauve via-ctp-lavender to-ctp-blue bg-[length:200%_auto] px-4 py-4 text-center text-lg font-semibold text-ctp-base shadow-lg shadow-ctp-mauve/20 transition-all duration-300 hover:bg-right hover:shadow-xl hover:shadow-ctp-mauve/40 enabled:hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-40 disabled:shadow-none animate-fade-up"
      style="animation-delay: 240ms"
      disabled={!canSubmit}
      onclick={submit}
    >
      Optimize
      {#if files.length > 0 && selectedPresets.length > 0}
        ({files.length} file{files.length === 1 ? '' : 's'} × {selectedPresets.length} preset{selectedPresets.length ===
        1
          ? ''
          : 's'})
      {/if}
    </button>
  {/if}

  {#if busy || status === 'done' || status === 'error'}
    <section class="mx-auto flex w-full max-w-5xl flex-col gap-4 animate-fade-up">
      {#if status === 'uploading'}
        <p class="flex items-center gap-2 text-ctp-text">
          <span class="inline-flex gap-1" aria-hidden="true">
            <span class="h-2 w-2 animate-bounce rounded-full bg-ctp-mauve [animation-delay:-0.3s]"></span>
            <span class="h-2 w-2 animate-bounce rounded-full bg-ctp-blue [animation-delay:-0.15s]"></span>
            <span class="h-2 w-2 animate-bounce rounded-full bg-ctp-sapphire"></span>
          </span>
          Uploading…
        </p>
      {/if}

      {#if status !== 'error'}
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {#each selectedPresets as name (name)}
            <ProgressCard
              {name}
              completed={progress.presetProgress[name]?.completed ?? 0}
              expected={progress.presetProgress[name]?.expected ?? 0}
            />
          {/each}
        </div>
      {/if}

      {#if status === 'done'}
        <div class="flex flex-col gap-1.5">
          <p class="text-sm text-ctp-subtext1">
            Your download should start automatically. If it didn’t, use the button below.
          </p>
          <a
            class="group inline-flex animate-pop items-center gap-2 self-start rounded-lg bg-gradient-to-r from-ctp-green to-ctp-teal bg-[length:200%_auto] px-5 py-3 font-semibold text-ctp-base shadow-lg shadow-ctp-green/25 transition-all duration-300 hover:-translate-y-0.5 hover:bg-right hover:shadow-xl hover:shadow-ctp-green/40"
            href={progress.downloadUrl}
            download
          >
            <svg
              class="h-5 w-5 transition-transform duration-300 group-hover:translate-y-0.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
              aria-hidden="true"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
            </svg>
            {singleImageResult ? 'Download image' : 'Download ZIP'}
          </a>
        </div>
      {:else if status === 'error'}
        <p class="text-ctp-red">
          {progress.errorMessage ?? 'Something went wrong.'}
        </p>
      {/if}

      {#if status === 'done' || status === 'error'}
        <button
          type="button"
          class="group inline-flex items-center gap-1.5 self-start text-sm text-ctp-blue transition-colors hover:text-ctp-mauve"
          onclick={reset}
        >
          <svg
            class="h-4 w-4 transition-transform duration-500 group-hover:-rotate-180"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
            aria-hidden="true"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992V4.356M19.652 9.348a8.25 8.25 0 1 0 .9 7.4" />
          </svg>
          Start over
        </button>
      {/if}
    </section>
  {/if}

  <!-- Footer: sticks to the bottom (mt-auto) with legal links that open the
       in-app modal instead of navigating away. -->
  <footer
    class="mt-auto flex flex-wrap items-center justify-center gap-x-4 gap-y-1 border-t border-ctp-surface1 pt-6 text-sm text-ctp-overlay0"
  >
    <span>© Henry Rausch</span>
    <button type="button" class="hover:text-ctp-blue" onclick={() => (legalOpen = 'imprint')}>
      Imprint
    </button>
    <button type="button" class="hover:text-ctp-blue" onclick={() => (legalOpen = 'privacy')}>
      Privacy Policy
    </button>
  </footer>
</main>

<LegalModal bind:open={legalOpen} />
<CropModal bind:open={cropping} {selectedPresets} />
