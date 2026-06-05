<script>
  // PresetSelector: grouped checkbox grid of the output presets. The set of
  // selected preset names is bound back to the parent via `selected`.
  //
  // The preset registry lives in ../lib/presets.js (shared with ProgressCard).

  import { PRESET_GROUPS as GROUPS, PRESET_NAMES as ALL_NAMES } from '../lib/presets.js';

  let { selected = $bindable([]), disabled = false } = $props();

  let allSelected = $derived(selected.length === ALL_NAMES.length);

  function isSelected(name) {
    return selected.includes(name);
  }

  function toggle(name) {
    if (disabled) return;
    selected = isSelected(name)
      ? selected.filter((n) => n !== name)
      : [...selected, name];
  }

  function toggleAll() {
    if (disabled) return;
    selected = allSelected ? [] : [...ALL_NAMES];
  }
</script>

<div class="flex flex-col gap-4">
  <div class="flex items-center justify-between">
    <h2 class="text-sm font-semibold tracking-wide text-ctp-subtext1 uppercase">
      Presets
    </h2>
    <button
      type="button"
      class="text-sm font-medium text-ctp-blue hover:text-ctp-mauve disabled:opacity-50"
      {disabled}
      onclick={toggleAll}
    >
      {allSelected ? 'Deselect all' : 'Select all'}
    </button>
  </div>

  {#each GROUPS as group (group.category)}
    <fieldset class="flex flex-col gap-2">
      <legend class="mb-1 text-xs font-semibold tracking-wide text-ctp-overlay0 uppercase">
        {group.category}
      </legend>
      <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
        {#each group.presets as preset (preset.name)}
          <label
            class="flex cursor-pointer items-center gap-3 rounded-lg border p-3 transition-colors
                   {isSelected(preset.name)
              ? 'border-ctp-mauve bg-ctp-surface0'
              : 'border-ctp-surface1 bg-ctp-base hover:border-ctp-overlay0'}
                   {disabled ? 'cursor-not-allowed opacity-50' : ''}"
          >
            <input
              type="checkbox"
              class="h-4 w-4 flex-none accent-ctp-mauve"
              checked={isSelected(preset.name)}
              {disabled}
              onchange={() => toggle(preset.name)}
            />
            <span class="flex-1">
              <span class="block text-sm text-ctp-text">{preset.label}</span>
              <span class="block text-xs text-ctp-subtext1">{preset.dims}</span>
            </span>
          </label>
        {/each}
      </div>
    </fieldset>
  {/each}
</div>
