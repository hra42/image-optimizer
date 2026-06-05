<script>
  // PresetSelector: grouped checkbox grid of the output presets. The set of
  // selected preset names is bound back to the parent via `selected`.
  //
  // The preset registry lives in ../lib/presets.js (shared with ProgressCard).

  import { PRESET_GROUPS as GROUPS } from '../lib/presets.js';

  let { selected = $bindable([]), disabled = false } = $props();

  function isSelected(name) {
    return selected.includes(name);
  }

  function toggle(name) {
    if (disabled) return;
    selected = isSelected(name)
      ? selected.filter((n) => n !== name)
      : [...selected, name];
  }
</script>

<div class="flex flex-col gap-4">
  <div class="min-w-0">
    <h2 class="text-base font-semibold tracking-wide text-ctp-subtext1 uppercase">
      Presets
    </h2>
    <p class="text-sm text-ctp-overlay0">
      Pick one or more — each makes its own optimized copy of every image.
    </p>
  </div>

  {#each GROUPS as group (group.category)}
    <fieldset class="flex flex-col gap-2">
      <legend class="mb-1 text-sm font-semibold tracking-wide text-ctp-overlay0 uppercase">
        {group.category}
      </legend>
      <!-- Pills wrap and size to their content rather than stretching to fill
           equal grid columns, so a short label like "WebP" stays compact. -->
      <div class="flex flex-wrap gap-2">
        {#each group.presets as preset (preset.name)}
          <label
            class="flex cursor-pointer items-center gap-3 rounded-lg border px-4 py-3 transition-colors
                   {isSelected(preset.name)
              ? 'border-ctp-mauve bg-ctp-surface0'
              : 'border-ctp-surface1 bg-ctp-base hover:border-ctp-overlay0'}
                   {disabled ? 'cursor-not-allowed opacity-50' : ''}"
          >
            <input
              type="checkbox"
              class="h-5 w-5 flex-none accent-ctp-mauve"
              checked={isSelected(preset.name)}
              {disabled}
              onchange={() => toggle(preset.name)}
            />
            <span>
              <span class="block text-base text-ctp-text">{preset.label}</span>
              <span class="block text-sm text-ctp-subtext1">{preset.dims}</span>
            </span>
          </label>
        {/each}
      </div>
    </fieldset>
  {/each}
</div>
