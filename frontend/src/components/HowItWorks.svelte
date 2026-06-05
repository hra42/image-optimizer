<script>
  // HowItWorks: a compact onboarding strip shown on the idle screen so a
  // first-time user understands the flow before touching anything. Purely
  // presentational — no props, no state.

  const steps = [
    {
      n: 1,
      title: 'Add images',
      body: 'Drag & drop or browse. JPEG, PNG, WebP, AVIF, and iPhone HEIC are all supported. Add as many as you like.',
    },
    {
      n: 2,
      title: 'Pick targets',
      body: 'Choose one or more presets — website formats, social sizes (Instagram, LinkedIn, X…), banners, or the Favicon pack (a full drop-in icon set).',
    },
    {
      n: 3,
      title: 'Optimize',
      body: 'Each image is resized and re-encoded for every preset you picked. Watch live progress per target.',
    },
    {
      n: 4,
      title: 'Download',
      body: 'Grab a single ZIP with every optimized variant, named by preset.',
    },
  ];
</script>

<section
  class="flex flex-col gap-3 rounded-xl border border-ctp-surface1 bg-ctp-surface0 p-4"
  aria-label="How it works"
>
  <h2 class="text-sm font-semibold tracking-wide text-ctp-overlay0 uppercase">
    How it works
  </h2>
  <ol class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
    {#each steps as step (step.n)}
      <li class="flex items-start gap-3">
        <span
          class="flex h-7 w-7 flex-none items-center justify-center rounded-full bg-ctp-mauve text-sm font-bold text-ctp-base"
          aria-hidden="true"
        >
          {step.n}
        </span>
        <span class="min-w-0">
          <span class="block text-base font-medium text-ctp-text">{step.title}</span>
          <span class="block text-sm leading-relaxed text-ctp-subtext1">{step.body}</span>
        </span>
      </li>
    {/each}
  </ol>
  <p class="text-sm text-ctp-overlay0">
    Everything runs in memory and is discarded after you download — nothing is
    stored on disk.
  </p>
</section>
