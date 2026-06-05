<script>
  // ProgressCard: one card per selected preset. Shows the preset's label +
  // target dimensions and an animated bar that fills as the preset's files
  // report in. Ticks to a green checkmark at 100%, or turns red on failure.
  //
  // The bar fills in the category's accent color (via PRESET_META.accent) and
  // sweeps a shimmer while in flight; it flips to green on done / red on fail.
  //
  // Read-only props (no $bindable): the parent owns the counts via the progress
  // store. `completed`/`expected` are this preset's own units, not the whole job.

  import { PRESET_META } from '../lib/presets.js';

  let { name, completed = 0, expected = 0, failed = false } = $props();

  let meta = $derived(PRESET_META[name] ?? { label: name, dims: '', accent: 'mauve' });
  let pct = $derived(
    expected > 0 ? Math.round((completed / expected) * 100) : 0,
  );
  let done = $derived(!failed && expected > 0 && completed >= expected);
  let active = $derived(!failed && !done && completed > 0);

  // Static accent → gradient classes (Tailwind v4 can't see interpolated names).
  const BARS = {
    mauve: 'from-ctp-mauve to-ctp-pink',
    blue: 'from-ctp-blue to-ctp-sapphire',
    pink: 'from-ctp-pink to-ctp-mauve',
    sapphire: 'from-ctp-sapphire to-ctp-sky',
    sky: 'from-ctp-sky to-ctp-teal',
    lavender: 'from-ctp-lavender to-ctp-blue',
    red: 'from-ctp-red to-ctp-peach',
    teal: 'from-ctp-teal to-ctp-green',
    yellow: 'from-ctp-yellow to-ctp-peach',
    peach: 'from-ctp-peach to-ctp-yellow',
  };
  let barClass = $derived(
    failed
      ? 'bg-ctp-red'
      : done
        ? 'bg-gradient-to-r from-ctp-green to-ctp-teal'
        : `bg-gradient-to-r ${BARS[meta.accent] ?? BARS.mauve}`,
  );
</script>

<div
  class="flex flex-col gap-2 rounded-lg border bg-ctp-surface0 p-3 transition-all duration-300
         {done
    ? 'border-ctp-green/50 shadow-md shadow-ctp-green/10'
    : failed
      ? 'border-ctp-red/50'
      : 'border-ctp-surface1'}"
>
  <div class="flex items-baseline justify-between gap-2">
    <div class="min-w-0">
      <span class="block truncate text-sm font-medium text-ctp-text">{meta.label}</span>
      <span class="block text-xs text-ctp-subtext1">{meta.dims}</span>
    </div>

    {#if failed}
      <span class="flex-none text-sm font-semibold text-ctp-red">Failed</span>
    {:else if done}
      <span
        class="flex aspect-square h-5 w-5 flex-none animate-pop items-center justify-center rounded-full bg-ctp-green text-xs font-bold text-ctp-base"
        aria-label="Complete"
      >✓</span>
    {:else}
      <span class="flex-none text-xs tabular-nums text-ctp-subtext1">{pct}%</span>
    {/if}
  </div>

  <div class="relative h-2 w-full overflow-hidden rounded-full bg-ctp-surface1">
    <div
      class="h-full rounded-full bg-[length:200%_auto] transition-all duration-500 ease-out {barClass}"
      style="width: {failed ? 100 : pct}%"
    ></div>
    <!-- Shimmer sweep, only while the bar is actively filling. -->
    {#if active}
      <div class="bar-shimmer pointer-events-none absolute inset-0 rounded-full"></div>
    {/if}
  </div>
</div>
