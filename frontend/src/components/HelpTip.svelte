<script>
  // HelpTip: a small circular "?" button that reveals a short help popover.
  //
  // Desktop: shows on hover and on keyboard focus. Mobile/touch: tap toggles it.
  // The button lives inside the preset <label>, so it must NOT toggle the
  // checkbox — every handler stops propagation and prevents default.
  //
  // Accessibility: the popover is linked via aria-describedby and the button is
  // a real <button> (focusable, Enter/Space activate). Escape closes it.

  let { text = '', label = 'More info' } = $props();

  let open = $state(false);
  let hovered = $state(false);
  let focused = $state(false);
  let id = `helptip-${Math.random().toString(36).slice(2, 9)}`;

  // Visible when hovered, keyboard-focused, or tapped open.
  let show = $derived(open || hovered || focused);

  function toggle(e) {
    e.preventDefault();
    e.stopPropagation();
    open = !open;
  }

  function onKey(e) {
    if (e.key === 'Escape') {
      open = false;
      focused = false;
    }
  }

  // Swallow the label's click so tapping ? never toggles the checkbox.
  function swallow(e) {
    e.preventDefault();
    e.stopPropagation();
  }
</script>

<span class="relative inline-flex" onclick={swallow} onkeydown={onKey} role="presentation">
  <button
    type="button"
    class="flex h-5 w-5 flex-none items-center justify-center rounded-full border border-ctp-surface1 text-xs font-bold text-ctp-overlay0 transition-colors duration-200 hover:border-ctp-mauve hover:text-ctp-mauve focus-visible:ring-2 focus-visible:ring-ctp-mauve focus-visible:outline-none"
    aria-label={label}
    aria-expanded={show}
    aria-describedby={show ? id : undefined}
    onclick={toggle}
    onpointerenter={() => (hovered = true)}
    onpointerleave={() => (hovered = false)}
    onfocus={() => (focused = true)}
    onblur={() => (focused = false)}
  >
    ?
  </button>

  {#if show}
    <span
      {id}
      role="tooltip"
      class="animate-fade-up absolute bottom-full left-1/2 z-20 mb-2 w-60 -translate-x-1/2 rounded-lg border border-ctp-surface1 bg-ctp-surface0 px-3 py-2 text-left text-xs leading-relaxed font-normal text-ctp-subtext1 shadow-xl"
    >
      {text}
      <!-- Little pointer triangle. -->
      <span
        class="absolute top-full left-1/2 -mt-px h-2 w-2 -translate-x-1/2 rotate-45 border-r border-b border-ctp-surface1 bg-ctp-surface0"
        aria-hidden="true"
      ></span>
    </span>
  {/if}
</span>
