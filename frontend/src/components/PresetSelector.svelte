<script>
  // PresetSelector: grouped checkbox grid of the output presets. The set of
  // selected preset names is bound back to the parent via `selected`.
  //
  // The preset registry lives in ../lib/presets.js (shared with ProgressCard).

  import { PRESET_GROUPS as GROUPS } from '../lib/presets.js';
  import HelpTip from './HelpTip.svelte';
  import FormatGuide from './FormatGuide.svelte';

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

  // Tailwind v4's JIT only emits classes whose full strings appear literally in
  // source, so per-accent classes can't be built by interpolation. Map each
  // accent token to its static class strings: `legend` tints the group heading,
  // `on` styles the selected pill, `off` is the resting hover border, and
  // `check` fills the custom checkbox indicator (border + bg) when its peer
  // input is checked. Every class here is a literal the scanner can see.
  const ACCENTS = {
    mauve:    { legend: 'text-ctp-mauve',    on: 'border-ctp-mauve bg-ctp-mauve/10 shadow-md shadow-ctp-mauve/20',       off: 'hover:border-ctp-mauve/60',    check: 'peer-checked:border-ctp-mauve peer-checked:bg-ctp-mauve' },
    blue:     { legend: 'text-ctp-blue',     on: 'border-ctp-blue bg-ctp-blue/10 shadow-md shadow-ctp-blue/20',          off: 'hover:border-ctp-blue/60',     check: 'peer-checked:border-ctp-blue peer-checked:bg-ctp-blue' },
    pink:     { legend: 'text-ctp-pink',     on: 'border-ctp-pink bg-ctp-pink/10 shadow-md shadow-ctp-pink/20',          off: 'hover:border-ctp-pink/60',     check: 'peer-checked:border-ctp-pink peer-checked:bg-ctp-pink' },
    sapphire: { legend: 'text-ctp-sapphire', on: 'border-ctp-sapphire bg-ctp-sapphire/10 shadow-md shadow-ctp-sapphire/20', off: 'hover:border-ctp-sapphire/60', check: 'peer-checked:border-ctp-sapphire peer-checked:bg-ctp-sapphire' },
    sky:      { legend: 'text-ctp-sky',      on: 'border-ctp-sky bg-ctp-sky/10 shadow-md shadow-ctp-sky/20',             off: 'hover:border-ctp-sky/60',      check: 'peer-checked:border-ctp-sky peer-checked:bg-ctp-sky' },
    lavender: { legend: 'text-ctp-lavender', on: 'border-ctp-lavender bg-ctp-lavender/10 shadow-md shadow-ctp-lavender/20', off: 'hover:border-ctp-lavender/60', check: 'peer-checked:border-ctp-lavender peer-checked:bg-ctp-lavender' },
    red:      { legend: 'text-ctp-red',      on: 'border-ctp-red bg-ctp-red/10 shadow-md shadow-ctp-red/20',             off: 'hover:border-ctp-red/60',      check: 'peer-checked:border-ctp-red peer-checked:bg-ctp-red' },
    teal:     { legend: 'text-ctp-teal',     on: 'border-ctp-teal bg-ctp-teal/10 shadow-md shadow-ctp-teal/20',          off: 'hover:border-ctp-teal/60',     check: 'peer-checked:border-ctp-teal peer-checked:bg-ctp-teal' },
    yellow:   { legend: 'text-ctp-yellow',   on: 'border-ctp-yellow bg-ctp-yellow/10 shadow-md shadow-ctp-yellow/20',    off: 'hover:border-ctp-yellow/60',   check: 'peer-checked:border-ctp-yellow peer-checked:bg-ctp-yellow' },
    peach:    { legend: 'text-ctp-peach',    on: 'border-ctp-peach bg-ctp-peach/10 shadow-md shadow-ctp-peach/20',       off: 'hover:border-ctp-peach/60',    check: 'peer-checked:border-ctp-peach peer-checked:bg-ctp-peach' },
    green:    { legend: 'text-ctp-green',    on: 'border-ctp-green bg-ctp-green/10 shadow-md shadow-ctp-green/20',       off: 'hover:border-ctp-green/60',    check: 'peer-checked:border-ctp-green peer-checked:bg-ctp-green' },
    flamingo: { legend: 'text-ctp-flamingo', on: 'border-ctp-flamingo bg-ctp-flamingo/10 shadow-md shadow-ctp-flamingo/20', off: 'hover:border-ctp-flamingo/60', check: 'peer-checked:border-ctp-flamingo peer-checked:bg-ctp-flamingo' },
  };

  function accentOf(name) {
    return ACCENTS[name] ?? ACCENTS.mauve;
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

  <FormatGuide />

  {#each GROUPS as group (group.category)}
    {@const a = accentOf(group.accent)}
    <fieldset class="flex flex-col gap-2">
      <legend class="mb-1 flex items-center gap-2 text-sm font-semibold tracking-wide uppercase {a.legend}">
        <span class="h-2 w-2 rounded-full bg-current"></span>
        {group.category}
      </legend>
      <!-- Pills wrap and size to their content rather than stretching to fill
           equal grid columns, so a short label like "WebP" stays compact. -->
      <div class="flex flex-wrap gap-2">
        {#each group.presets as preset (preset.name)}
          {@const on = isSelected(preset.name)}
          <label
            class="flex cursor-pointer items-center gap-3 rounded-lg border px-4 py-3 transition-all duration-200 hover:-translate-y-0.5
                   {on ? a.on : `border-ctp-surface1 bg-ctp-base ${a.off}`}
                   {disabled ? 'cursor-not-allowed opacity-50' : ''}"
          >
            <!-- Real checkbox kept for a11y/keyboard but visually hidden; the
                 styled box next to it (driven by peer-checked) is what shows. -->
            <input
              type="checkbox"
              class="peer sr-only"
              checked={on}
              {disabled}
              onchange={() => toggle(preset.name)}
            />
            <span
              class="flex h-5 w-5 flex-none items-center justify-center rounded-md border-2 border-ctp-surface1 bg-ctp-base text-ctp-base transition-all duration-200 peer-hover:border-ctp-overlay0 peer-focus-visible:ring-2 peer-focus-visible:ring-ctp-overlay0 peer-focus-visible:ring-offset-1 peer-focus-visible:ring-offset-ctp-base {a.check}"
              aria-hidden="true"
            >
              <svg
                class="h-3.5 w-3.5 transition-all duration-200 {on ? 'scale-100 opacity-100' : 'scale-0 opacity-0'}"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="3.5"
              >
                <path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
              </svg>
            </span>
            <span class={on ? 'animate-pop' : ''}>
              <span class="block text-base text-ctp-text">{preset.label}</span>
              <span class="block text-sm text-ctp-subtext1">{preset.dims}</span>
            </span>
            {#if preset.help}
              <HelpTip text={preset.help} label="What is {preset.label}?" />
            {/if}
          </label>
        {/each}
      </div>
    </fieldset>
  {/each}
</div>
