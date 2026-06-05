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
  import { createProgress } from './stores/progress.svelte.js';

  const progress = createProgress();

  let files = $state([]); // [{ file, url }]
  let selectedPresets = $state([]); // preset name strings
  let legalOpen = $state(null); // 'imprint' | 'privacy' | null

  let status = $derived(progress.status);
  let canSubmit = $derived(
    status === 'idle' && files.length > 0 && selectedPresets.length > 0,
  );
  let busy = $derived(status === 'uploading' || status === 'processing');

  async function submit() {
    if (!canSubmit) return;

    progress.start(selectedPresets, files.length);

    const form = new FormData();
    for (const entry of files) form.append('files', entry.file);
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
  }

  // Ensure the stream is torn down if the component unmounts mid-flight.
  $effect(() => () => progress.close());
</script>

<main class="mx-auto flex min-h-screen w-full max-w-7xl flex-col gap-8 px-4 py-8 sm:px-6 lg:px-10 lg:py-12">
  <header class="flex flex-col gap-1">
    <h1 class="text-2xl font-bold text-ctp-mauve sm:text-3xl">Image Optimizer</h1>
    <p class="text-sm text-ctp-subtext1 sm:text-base">
      Drop images, pick targets, download optimized variants as a ZIP.
    </p>
  </header>

  {#if status === 'idle'}
    <!--
      Single full-width column in task order: how-it-works → drop → presets →
      optimize. The inner components (the how-it-works steps and the preset
      grid) spread horizontally to fill the width, so the page uses the whole
      container on desktop without a lopsided multi-column split.
    -->
    <HowItWorks />
    <Dropzone bind:files />
    <PresetSelector bind:selected={selectedPresets} />

    <button
      type="button"
      class="rounded-lg bg-ctp-mauve px-4 py-4 text-center text-lg font-semibold text-ctp-base transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
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
    <section class="mx-auto flex w-full max-w-5xl flex-col gap-4">
      {#if status === 'uploading'}
        <p class="text-ctp-text">Uploading…</p>
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
        <a
          class="self-start rounded-lg bg-ctp-green px-5 py-3 font-semibold text-ctp-base transition-opacity hover:opacity-90"
          href={progress.downloadUrl}
        >
          Download ZIP
        </a>
      {:else if status === 'error'}
        <p class="text-ctp-red">
          {progress.errorMessage ?? 'Something went wrong.'}
        </p>
      {/if}

      {#if status === 'done' || status === 'error'}
        <button
          type="button"
          class="self-start text-sm text-ctp-blue hover:text-ctp-mauve"
          onclick={reset}
        >
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
