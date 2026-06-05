<script>
  // LegalModal: an overlay that shows either the Imprint or the Privacy policy.
  // Controlled by the parent via `open` (the active doc key, or null). Closes on
  // backdrop click, the × button, or Escape. The legal text lives here as the
  // single source of truth; the footer just toggles which key is open.
  //
  // The privacy text is written specifically for THIS app (in-memory only, no
  // storage, TTL deletion, no cookies/tracking) — not copied from hra42.com,
  // whose policy describes analytics/hosting this app does not use.

  let { open = $bindable(null) } = $props();

  function close() {
    open = null;
  }

  function onKeydown(e) {
    if (e.key === 'Escape') close();
  }

  const title = $derived(open === 'imprint' ? 'Imprint' : 'Privacy Policy');
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
  <!-- Backdrop -->
  <div
    class="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-0 sm:items-center sm:p-4"
    role="dialog"
    aria-modal="true"
    aria-label={title}
    onclick={close}
  >
    <!-- Panel: stop propagation so clicks inside don't close it. -->
    <div
      class="flex max-h-[90vh] w-full max-w-2xl flex-col rounded-t-2xl border border-ctp-surface1 bg-ctp-base shadow-xl sm:rounded-2xl"
      onclick={(e) => e.stopPropagation()}
    >
      <header class="flex items-center justify-between border-b border-ctp-surface1 px-6 py-4">
        <h2 class="text-lg font-bold text-ctp-mauve">{title}</h2>
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

      <div class="overflow-y-auto px-6 py-5 text-sm leading-relaxed text-ctp-subtext1">
        {#if open === 'imprint'}
          <h3 class="mb-1 text-base font-semibold text-ctp-text">
            Information according to § 5 TMG
          </h3>
          <p class="mb-4">
            Henry Rausch<br />
            Postfach 30324521<br />
            39047 Magdeburg<br />
            Germany
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">Contact</h3>
          <p class="mb-4">
            Email:
            <a class="text-ctp-blue hover:text-ctp-mauve" href="mailto:image-transformer@hra42.com">
              image-transformer@hra42.com
            </a>
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">
            Responsible for content
          </h3>
          <p class="mb-4">Henry Rausch (address as above)</p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">Liability for content</h3>
          <p class="mb-4">
            As a service provider we are responsible for our own content on these
            pages in accordance with general law. We are not obliged to monitor
            transmitted or stored third-party information or to investigate
            circumstances that indicate illegal activity.
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">Liability for links</h3>
          <p>
            Our pages may contain links to external websites over whose content we
            have no influence. We therefore cannot accept any liability for this
            third-party content. The respective provider or operator of the linked
            pages is always responsible for their content.
          </p>
        {:else}
          <p class="mb-4 text-ctp-overlay0">Last updated: 5 June 2026</p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">In short</h3>
          <p class="mb-4">
            This tool optimizes your images entirely in memory. Your uploads are
            <strong class="text-ctp-text">never written to disk</strong>, never shared,
            and are deleted automatically. There are no accounts, no analytics, and
            no advertising or cross-site tracking.
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">Controller</h3>
          <p class="mb-4">
            Henry Rausch, Postfach 30324521, 39047 Magdeburg, Germany —
            <a class="text-ctp-blue hover:text-ctp-mauve" href="mailto:image-transformer@hra42.com">
              image-transformer@hra42.com
            </a>
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">
            What happens to images you upload
          </h3>
          <p class="mb-4">
            When you upload images, they are held in server memory (RAM) only and
            processed into the formats you selected. The resulting files are bundled
            into a ZIP for you to download. Neither your originals nor the optimized
            outputs are stored on disk or in any database.
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">How long data is kept</h3>
          <p class="mb-4">
            A job's data is removed from memory as soon as you download the ZIP, or
            automatically after a short time-to-live (10 minutes by default) if you
            never download it — whichever comes first. After that, nothing remains.
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">
            Cookies, tracking & analytics
          </h3>
          <p class="mb-4">
            The application sets no cookies of its own, embeds no third-party
            scripts, and runs no analytics or tracking. It does not respond to
            Do-Not-Track signals because it does not track in the first place.
          </p>
          <p class="mb-4">
            However, our security and delivery provider Cloudflare (see “Hosting &amp;
            infrastructure” below) sets a small number of
            <strong class="text-ctp-text">strictly necessary cookies</strong> to
            protect the site — for example
            <code class="text-ctp-text">__cf_bm</code> (bot management, ~30 minutes)
            and <code class="text-ctp-text">cf_clearance</code> (security challenge
            verification, ~1 day), and, where load balancing is used,
            <code class="text-ctp-text">__cflb</code>. These cookies are required for
            Cloudflare's security features to work and contain no advertising or
            cross-site tracking data. Because they are strictly necessary, they do
            not require consent under the ePrivacy Directive.
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">Server logs</h3>
          <p class="mb-4">
            Like any web server, the host may process technical request data (such
            as IP address and timestamp) transiently to deliver responses and
            protect against abuse. This is not used to profile you and is not
            combined with your image uploads.
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">
            Hosting &amp; infrastructure
          </h3>
          <p class="mb-4">
            The application is hosted on servers provided by
            <strong class="text-ctp-text">Hetzner Online GmbH</strong> (Industriestr. 25,
            91710 Gunzenhausen, Germany), with data centres located in the EU. Hetzner
            acts as a processor on our behalf under a data processing agreement.
          </p>
          <p class="mb-4">
            Incoming traffic is routed through
            <strong class="text-ctp-text">Cloudflare</strong> (Cloudflare, Inc., 101
            Townsend St., San Francisco, CA 94107, USA), which we use as a reverse
            proxy and content delivery network for security (e.g. DDoS protection,
            TLS) and performance. To provide these services, Cloudflare processes
            connection metadata such as your IP address and request headers. Because
            Cloudflare is a US provider, this may involve a transfer of data to the
            United States; such transfers are safeguarded by the EU Standard
            Contractual Clauses. See
            <a
              class="text-ctp-blue hover:text-ctp-mauve"
              href="https://www.cloudflare.com/privacypolicy/"
              target="_blank"
              rel="noopener noreferrer"
            >
              Cloudflare's privacy policy
            </a>
            for details. Your uploaded images are processed only in our application's
            memory and are not stored by Cloudflare or Hetzner beyond the transient
            handling described above.
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">Your rights</h3>
          <p class="mb-4">
            Under the GDPR you have the right to access, rectification, erasure, and
            restriction of processing. Because images are deleted automatically and
            nothing is stored, most requests resolve themselves — but you can reach
            out any time at
            <a class="text-ctp-blue hover:text-ctp-mauve" href="mailto:image-transformer@hra42.com">
              image-transformer@hra42.com
            </a>.
          </p>

          <h3 class="mb-1 text-base font-semibold text-ctp-text">Minors</h3>
          <p>
            This service is not directed at children under 18 and does not knowingly
            collect data from them.
          </p>
        {/if}
      </div>
    </div>
  </div>
{/if}
