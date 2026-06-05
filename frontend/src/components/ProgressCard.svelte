<script>
  // ProgressCard: one card per selected preset. Shows the preset's label +
  // target dimensions and an animated bar that fills as the preset's files
  // report in. Ticks to a green checkmark at 100%, or turns red on failure.
  //
  // Read-only props (no $bindable): the parent owns the counts via the progress
  // store. `completed`/`expected` are this preset's own units, not the whole job.

  import { PRESET_META } from '../lib/presets.js';

  let { name, completed = 0, expected = 0, failed = false } = $props();

  let meta = $derived(PRESET_META[name] ?? { label: name, dims: '' });
  let pct = $derived(
    expected > 0 ? Math.round((completed / expected) * 100) : 0,
  );
  let done = $derived(!failed && expected > 0 && completed >= expected);
</script>

<div class="flex flex-col gap-2 rounded-lg border border-ctp-surface1 bg-ctp-surface0 p-3">
  <div class="flex items-baseline justify-between gap-2">
    <div class="min-w-0">
      <span class="block truncate text-sm font-medium text-ctp-text">{meta.label}</span>
      <span class="block text-xs text-ctp-subtext1">{meta.dims}</span>
    </div>

    {#if failed}
      <span class="flex-none text-sm font-semibold text-ctp-red">Failed</span>
    {:else if done}
      <span class="flex-none text-sm font-semibold text-ctp-green" aria-label="Complete">✓</span>
    {:else}
      <span class="flex-none text-xs tabular-nums text-ctp-subtext1">{pct}%</span>
    {/if}
  </div>

  <div class="h-2 w-full overflow-hidden rounded-full bg-ctp-surface1">
    <div
      class="h-full transition-all duration-300 {failed ? 'bg-ctp-red' : 'bg-ctp-green'}"
      style="width: {failed ? 100 : pct}%"
    ></div>
  </div>
</div>
