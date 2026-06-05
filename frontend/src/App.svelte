<script>
  // App orchestrates the upload flow: idle → uploading → processing → done.
  // On submit it POSTs the files + selected presets to /upload, gets a jobId,
  // then opens an SSE stream to /progress/:jobId.
  //
  // SCAFFOLD (HRA-165): the progress display and download UI below are a
  // deliberately minimal placeholder. HRA-166 replaces them with the
  // stores/progress.js writable store + ProgressCard component + a styled
  // download button. App.svelte's job here is to drive the state machine and
  // own the EventSource lifecycle.

  import Dropzone from './components/Dropzone.svelte';
  import PresetSelector from './components/PresetSelector.svelte';

  // 'idle' | 'uploading' | 'processing' | 'done' | 'error'
  let status = $state('idle');
  let files = $state([]); // [{ file, url }]
  let selectedPresets = $state([]); // preset name strings

  let total = $state(0); // expected processing events = files * presets
  let completed = $state(0); // processing events seen so far
  let downloadUrl = $state(null);
  let errorMessage = $state(null);

  let eventSource = null;

  let canSubmit = $derived(
    status === 'idle' && files.length > 0 && selectedPresets.length > 0,
  );
  let busy = $derived(status === 'uploading' || status === 'processing');

  function closeStream() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  function fail(message) {
    closeStream();
    errorMessage = message;
    status = 'error';
  }

  async function submit() {
    if (!canSubmit) return;

    errorMessage = null;
    completed = 0;
    total = files.length * selectedPresets.length;
    status = 'uploading';

    const form = new FormData();
    for (const entry of files) form.append('files', entry.file);
    for (const name of selectedPresets) form.append('presets', name);

    let jobId;
    try {
      const res = await fetch('/upload', { method: 'POST', body: form });
      if (!res.ok) {
        fail((await res.text()) || `Upload failed (${res.status})`);
        return;
      }
      ({ jobId } = await res.json());
    } catch (err) {
      fail(err instanceof Error ? err.message : 'Upload failed');
      return;
    }

    status = 'processing';
    connect(jobId);
  }

  function connect(jobId) {
    closeStream();
    eventSource = new EventSource(`/progress/${jobId}`);

    eventSource.onmessage = (e) => {
      let event;
      try {
        event = JSON.parse(e.data);
      } catch {
        return;
      }

      if (event.status === 'complete') {
        downloadUrl = event.downloadUrl ?? `/download/${jobId}`;
        status = 'done';
        closeStream();
      } else if (event.status === 'error') {
        fail('Processing failed');
      } else if (event.status === 'processing') {
        completed += 1;
      }
    };

    eventSource.onerror = () => {
      // A clean close after completion also fires onerror; ignore once done.
      if (status === 'processing') fail('Lost connection to the server');
    };
  }

  function reset() {
    closeStream();
    status = 'idle';
    files = [];
    selectedPresets = [];
    total = 0;
    completed = 0;
    downloadUrl = null;
    errorMessage = null;
  }

  // Ensure the stream is torn down if the component unmounts mid-flight.
  $effect(() => () => closeStream());
</script>

<main class="mx-auto flex min-h-screen max-w-2xl flex-col gap-8 px-4 py-10">
  <header class="flex flex-col gap-1">
    <h1 class="text-2xl font-bold text-ctp-mauve">Image Optimizer</h1>
    <p class="text-sm text-ctp-subtext1">
      Drop images, pick targets, get optimized variants.
    </p>
  </header>

  {#if status === 'idle'}
    <Dropzone bind:files />
    <PresetSelector bind:selected={selectedPresets} />

    <button
      type="button"
      class="rounded-lg bg-ctp-mauve px-4 py-3 font-semibold text-ctp-base transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
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
    <!-- SCAFFOLD placeholder — replaced by ProgressCard + download UI in HRA-166. -->
    <section
      class="flex flex-col gap-4 rounded-xl border border-ctp-surface1 bg-ctp-surface0 p-6"
    >
      {#if status === 'uploading'}
        <p class="text-ctp-text">Uploading…</p>
      {:else if status === 'processing'}
        <p class="text-ctp-text">Processing… {completed}/{total}</p>
        <div class="h-2 w-full overflow-hidden rounded-full bg-ctp-surface1">
          <div
            class="h-full bg-ctp-green transition-all"
            style="width: {total > 0 ? (completed / total) * 100 : 0}%"
          ></div>
        </div>
      {:else if status === 'done'}
        <p class="text-ctp-green">Done — {completed}/{total} variants ready.</p>
        <a
          class="self-start rounded-lg bg-ctp-green px-4 py-2 font-semibold text-ctp-base hover:opacity-90"
          href={downloadUrl}
        >
          Download ZIP
        </a>
      {:else if status === 'error'}
        <p class="text-ctp-red">{errorMessage ?? 'Something went wrong.'}</p>
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
</main>
