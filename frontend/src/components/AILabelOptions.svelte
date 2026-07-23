<script>
  let {
    enabled = $bindable(false),
    style = $bindable('generated'),
    color = $bindable('black'),
    position = $bindable('bottom-right'),
  } = $props();

  const styles = [
    { value: 'generated', label: 'AI generated' },
    { value: 'modified', label: 'AI modified' },
    { value: 'ai', label: 'AI icon' },
  ];
  const positions = [
    { value: 'top-left', label: 'Top left' },
    { value: 'top-right', label: 'Top right' },
    { value: 'bottom-left', label: 'Bottom left' },
    { value: 'bottom-right', label: 'Bottom right' },
  ];
</script>

<section class="overflow-hidden rounded-xl border border-ctp-surface1 bg-ctp-base transition-colors {enabled ? 'border-ctp-teal/70' : ''}">
  <label class="flex cursor-pointer items-center gap-4 px-4 py-4 sm:px-5">
    <input class="peer sr-only" type="checkbox" bind:checked={enabled} />
    <span
      class="relative h-7 w-12 flex-none rounded-full bg-ctp-surface1 transition-colors peer-checked:bg-ctp-teal peer-focus-visible:ring-2 peer-focus-visible:ring-ctp-teal peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-ctp-base after:absolute after:left-1 after:top-1 after:h-5 after:w-5 after:rounded-full after:bg-ctp-text after:transition-transform peer-checked:after:translate-x-5"
      aria-hidden="true"
    ></span>
    <span class="min-w-0">
      <span class="block font-semibold text-ctp-text">Add an AI label</span>
      <span class="block text-sm text-ctp-subtext1">
        Add a visible disclosure to regular image outputs. Favicons and PDFs are left unchanged.
      </span>
    </span>
  </label>

  {#if enabled}
    <div class="grid gap-5 border-t border-ctp-surface1 bg-ctp-surface0/35 px-4 py-5 sm:grid-cols-[1fr_auto] sm:px-5">
      <div class="flex min-w-0 flex-col gap-5">
        <fieldset>
          <legend class="mb-2 text-xs font-semibold tracking-wide text-ctp-teal uppercase">Label</legend>
          <div class="flex flex-wrap gap-2">
            {#each styles as option (option.value)}
              <label class="cursor-pointer">
                <input class="peer sr-only" type="radio" bind:group={style} value={option.value} />
                <span class="block rounded-lg border border-ctp-surface1 bg-ctp-base px-3 py-2 text-sm text-ctp-subtext1 transition-all peer-checked:border-ctp-teal peer-checked:bg-ctp-teal/10 peer-checked:text-ctp-text peer-focus-visible:ring-2 peer-focus-visible:ring-ctp-teal">
                  {option.label}
                </span>
              </label>
            {/each}
          </div>
        </fieldset>

        <div class="grid gap-5 sm:grid-cols-2">
          <fieldset>
            <legend class="mb-2 text-xs font-semibold tracking-wide text-ctp-teal uppercase">Color</legend>
            <div class="flex gap-2">
              {#each ['black', 'white'] as option}
                <label class="cursor-pointer">
                  <input class="peer sr-only" type="radio" bind:group={color} value={option} />
                  <span class="flex items-center gap-2 rounded-lg border border-ctp-surface1 bg-ctp-base px-3 py-2 text-sm capitalize text-ctp-subtext1 transition-all peer-checked:border-ctp-teal peer-checked:text-ctp-text peer-focus-visible:ring-2 peer-focus-visible:ring-ctp-teal">
                    <span class="h-3.5 w-3.5 rounded-full border border-ctp-overlay0 {option === 'black' ? 'bg-black' : 'bg-white'}"></span>
                    {option}
                  </span>
                </label>
              {/each}
            </div>
          </fieldset>

          <fieldset>
            <legend class="mb-2 text-xs font-semibold tracking-wide text-ctp-teal uppercase">Position</legend>
            <div class="grid w-fit grid-cols-2 gap-1.5" aria-label="Label position">
              {#each positions as option}
                <label class="cursor-pointer" title={option.label}>
                  <input class="peer sr-only" type="radio" bind:group={position} value={option.value} />
                  <span class="relative block h-8 w-10 rounded-md border border-ctp-surface1 bg-ctp-base transition-all peer-checked:border-ctp-teal peer-checked:bg-ctp-teal/15 peer-focus-visible:ring-2 peer-focus-visible:ring-ctp-teal">
                    <span class="absolute h-2 w-3 rounded-sm {position === option.value ? 'bg-ctp-teal' : 'bg-ctp-overlay0'} {option.value.includes('top') ? 'top-1' : 'bottom-1'} {option.value.includes('left') ? 'left-1' : 'right-1'}"></span>
                    <span class="sr-only">{option.label}</span>
                  </span>
                </label>
              {/each}
            </div>
          </fieldset>
        </div>
      </div>

      <div class="flex items-center justify-center sm:w-56">
        <div class="relative aspect-[4/3] w-full max-w-56 overflow-hidden rounded-lg border border-ctp-surface1 bg-gradient-to-br from-ctp-blue/70 via-ctp-lavender/60 to-ctp-pink/70 shadow-inner">
          <img
            src="/stamps/{style}-{color}.png"
            alt="Selected AI label preview"
            class="absolute max-h-[18%] max-w-[42%] object-contain {position.includes('top') ? 'top-[4%]' : 'bottom-[4%]'} {position.includes('left') ? 'left-[4%]' : 'right-[4%]'}"
          />
        </div>
      </div>
    </div>
  {/if}
</section>
