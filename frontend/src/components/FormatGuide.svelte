<script>
  // FormatGuide: a collapsible "Which format should I choose?" panel shown above
  // the preset grid. Collapsed by default so it doesn't clutter the page; uses
  // native <details>/<summary> so it works without JS and is keyboard/screen-
  // reader accessible out of the box.

  // Each row: format name, its accent color token, and a one-line "when to use".
  // Accent classes are written as literals so Tailwind's JIT emits them.
  const formats = [
    { name: 'WebP', dot: 'bg-ctp-blue', when: 'Best all-round choice. Small files, great quality, supported everywhere modern. If you’re unsure, pick this.' },
    { name: 'AVIF', dot: 'bg-ctp-teal', when: 'Smallest files of all — newest format. Ideal for maximum savings; pair with JPEG for older browsers.' },
    { name: 'JPEG', dot: 'bg-ctp-peach', when: 'The universal fallback. Perfect for photos, opens anywhere. No transparency.' },
    { name: 'PNG', dot: 'bg-ctp-green', when: 'Lossless with transparency. Best for logos, icons, screenshots, and sharp text.' },
  ];
</script>

<details
  class="group rounded-xl border border-ctp-surface1 bg-gradient-to-br from-ctp-surface0 to-ctp-base"
>
  <summary
    class="flex cursor-pointer list-none items-center gap-2 rounded-xl px-4 py-3 text-sm font-semibold text-ctp-text transition-colors hover:text-ctp-mauve focus-visible:ring-2 focus-visible:ring-ctp-mauve focus-visible:outline-none"
  >
    <!-- Chevron rotates when the panel opens. -->
    <svg
      class="h-4 w-4 flex-none text-ctp-mauve transition-transform duration-200 group-open:rotate-90"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2.5"
      aria-hidden="true"
    >
      <path stroke-linecap="round" stroke-linejoin="round" d="m9 5 7 7-7 7" />
    </svg>
    Which format should I choose?
  </summary>

  <div class="flex flex-col gap-3 px-4 pt-1 pb-4">
    <ul class="flex flex-col gap-2.5">
      {#each formats as f (f.name)}
        <li class="flex items-start gap-2.5">
          <span class="mt-1.5 h-2 w-2 flex-none rounded-full {f.dot}" aria-hidden="true"></span>
          <span class="min-w-0 text-sm leading-relaxed text-ctp-subtext1">
            <span class="font-semibold text-ctp-text">{f.name}</span>
            — {f.when}
          </span>
        </li>
      {/each}
    </ul>

    <p class="rounded-lg bg-ctp-base/60 px-3 py-2 text-sm leading-relaxed text-ctp-overlay0">
      <span class="font-semibold text-ctp-subtext1">Not sure?</span>
      Pick <span class="font-medium text-ctp-blue">WebP</span> for the web, or use a
      <span class="font-medium text-ctp-mauve">Convert</span> preset to change format while
      keeping the original size. Social presets (Instagram, LinkedIn, X…) resize to the exact
      dimensions each platform expects — hover any
      <span class="font-bold text-ctp-overlay0">?</span> for details.
    </p>
  </div>
</details>
