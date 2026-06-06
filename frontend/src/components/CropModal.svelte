<script>
  // CropModal: an opt-in per-image focal-point picker. The user drags a marker
  // onto the subject; fixed-aspect presets then crop around that point (server
  // side, via focalCrop) instead of guessing with attention. The default flow is
  // untouched — this only opens when the user clicks "Adjust crop" on a thumbnail.
  //
  // Coordinates are normalized to [0,1] against the displayed image, origin
  // top-left — exactly what the backend FocalPoint expects. The image is shown
  // upright (the browser bakes EXIF orientation for <img>), and the backend bakes
  // orientation before applying the focal, so the axes line up.
  //
  // Controlled by the parent via `open` (the file entry being adjusted, or null).
  // On save it writes `entry.focal = {x, y}` (or null on reset) back through the
  // bound entry. Reuses the LegalModal overlay pattern.

  import { cropAspects } from '../lib/presets.js';

  let { open = $bindable(null), selectedPresets = [] } = $props();

  // Working copy of the focal point while the modal is open: {x, y} or null.
  // Seeded from the entry's existing focal so re-opening keeps the prior nudge.
  let point = $state(null);
  let imgEl = $state(null);
  let dragging = $state(false);

  // Seed/clear the working point whenever a different entry is opened.
  $effect(() => {
    point = open?.focal ? { x: open.focal.x, y: open.focal.y } : null;
  });

  // The tightest fixed-aspect ratio among the selected presets, used to draw a
  // crop-box guide so the framing is legible. null when no selected preset crops.
  const aspect = $derived(firstCropAspect(selectedPresets));

  function firstCropAspect(names) {
    for (const name of names) {
      const a = cropAspects[name];
      if (a) return a; // { w, h }
    }
    return null;
  }

  function close() {
    open = null;
  }

  function onKeydown(e) {
    if (!open) return;
    if (e.key === 'Escape') close();
  }

  // Map a pointer event to a normalized [0,1] point against the rendered image.
  function pointFromEvent(e) {
    if (!imgEl) return null;
    const rect = imgEl.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return null;
    const x = (e.clientX - rect.left) / rect.width;
    const y = (e.clientY - rect.top) / rect.height;
    return { x: clamp01(x), y: clamp01(y) };
  }

  function clamp01(v) {
    return v < 0 ? 0 : v > 1 ? 1 : v;
  }

  function onPointerDown(e) {
    e.preventDefault();
    dragging = true;
    const p = pointFromEvent(e);
    if (p) point = p;
    imgEl?.setPointerCapture?.(e.pointerId);
  }

  function onPointerMove(e) {
    if (!dragging) return;
    const p = pointFromEvent(e);
    if (p) point = p;
  }

  function onPointerUp(e) {
    dragging = false;
    imgEl?.releasePointerCapture?.(e.pointerId);
  }

  function reset() {
    point = null;
  }

  function save() {
    if (open) open.focal = point ? { x: point.x, y: point.y } : null;
    close();
  }

  // Crop-box guide geometry as CSS percentages: a box of the target aspect,
  // centred on the focal point and clamped to the image, drawn over the image to
  // preview roughly what the crop keeps. Mirrors the server's cover-fill logic at
  // a coarse level (it assumes the displayed image's own aspect as the source).
  const guide = $derived(computeGuide(aspect, point, imgEl));

  function computeGuide(a, pt, el) {
    if (!a || !pt || !el) return null;
    const iw = el.clientWidth;
    const ih = el.clientHeight;
    if (iw === 0 || ih === 0) return null;
    // Box dimensions (in display px) of aspect a that fit inside the image with
    // the largest area — i.e. the crop window before cover-scaling.
    const target = a.w / a.h;
    const source = iw / ih;
    let bw, bh;
    if (target > source) {
      bw = iw;
      bh = iw / target;
    } else {
      bh = ih;
      bw = ih * target;
    }
    let left = pt.x * iw - bw / 2;
    let top = pt.y * ih - bh / 2;
    left = Math.max(0, Math.min(left, iw - bw));
    top = Math.max(0, Math.min(top, ih - bh));
    return {
      left: (left / iw) * 100,
      top: (top / ih) * 100,
      width: (bw / iw) * 100,
      height: (bh / ih) * 100,
    };
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-0 sm:items-center sm:p-4"
    role="dialog"
    aria-modal="true"
    aria-label="Adjust crop"
    onclick={close}
  >
    <!-- Panel: stop propagation so clicks inside don't close it. -->
    <div
      class="flex max-h-[90vh] w-full max-w-2xl flex-col rounded-t-2xl border border-ctp-surface1 bg-ctp-base shadow-xl sm:rounded-2xl"
      onclick={(e) => e.stopPropagation()}
    >
      <header class="flex items-center justify-between border-b border-ctp-surface1 px-6 py-4">
        <h2 class="text-lg font-bold text-ctp-mauve">Adjust crop</h2>
        <button
          type="button"
          class="rounded p-1 text-ctp-overlay0 transition-colors hover:text-ctp-text"
          aria-label="Close"
          onclick={close}
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
          </svg>
        </button>
      </header>

      <div class="overflow-y-auto px-6 py-5">
        <p class="mb-4 text-sm leading-relaxed text-ctp-subtext1">
          Drag the marker onto the most important part of
          <span class="font-medium text-ctp-text">{open.file.name}</span>. Presets that
          crop to a fixed shape will keep that point in frame.
          {#if !aspect}
            <span class="text-ctp-overlay0">
              None of your selected presets crop, so this won't change the output yet —
              pick a social or thumbnail preset to use it.
            </span>
          {/if}
        </p>

        <!-- Image stage: the marker and crop-box guide are positioned over the
             displayed image. touch-none keeps the browser from scrolling on drag. -->
        <div class="relative mx-auto w-full select-none">
          <img
            bind:this={imgEl}
            src={open.url}
            alt={open.file.name}
            draggable="false"
            class="mx-auto max-h-[55vh] w-auto cursor-crosshair touch-none rounded-lg ring-1 ring-ctp-surface1"
            onpointerdown={onPointerDown}
            onpointermove={onPointerMove}
            onpointerup={onPointerUp}
            onpointercancel={onPointerUp}
          />

          {#if guide}
            <!-- Dim everything outside the crop box with four edge overlays. -->
            <div
              class="pointer-events-none absolute inset-0"
              aria-hidden="true"
            >
              <div
                class="absolute rounded-sm ring-2 ring-ctp-mauve"
                style="left:{guide.left}%; top:{guide.top}%; width:{guide.width}%; height:{guide.height}%; box-shadow: 0 0 0 9999px rgba(0,0,0,0.45);"
              ></div>
            </div>
          {/if}

          {#if point}
            <!-- Focal marker. -->
            <div
              class="pointer-events-none absolute z-10 -translate-x-1/2 -translate-y-1/2"
              style="left:{point.x * 100}%; top:{point.y * 100}%;"
              aria-hidden="true"
            >
              <div class="h-5 w-5 rounded-full border-2 border-white bg-ctp-mauve shadow-lg shadow-black/40"></div>
            </div>
          {/if}
        </div>

        {#if !point}
          <p class="mt-3 text-center text-sm text-ctp-overlay0">
            Tap or click the image to set a focal point (otherwise smart auto-crop is used).
          </p>
        {/if}
      </div>

      <footer class="flex items-center justify-between gap-3 border-t border-ctp-surface1 px-6 py-4">
        <button
          type="button"
          class="text-sm text-ctp-subtext1 transition-colors hover:text-ctp-text disabled:opacity-40"
          disabled={!point}
          onclick={reset}
        >
          Reset to auto
        </button>
        <button
          type="button"
          class="rounded-lg bg-gradient-to-r from-ctp-mauve to-ctp-lavender px-4 py-2 text-sm font-semibold text-ctp-base shadow-md shadow-ctp-mauve/20 transition-all hover:-translate-y-0.5 hover:shadow-lg hover:shadow-ctp-mauve/40"
          onclick={save}
        >
          Save crop
        </button>
      </footer>
    </div>
  </div>
{/if}
