# Roadmap

## Context

The question on the table is strategic, not a single feature: *"Where else could we go with this project, or what should we add?"* — with two hard exclusions stated up front: **no API and no CLI** (those use-cases are better served by JS build tools like vite/sharp/squoosh and existing CLI tools like imagemagick).

The recent removal of AI background removal (commits `4037a7b` / `5d369bf`) is the defining signal. BiRefNet via ONNX Runtime was ripped out because a ~224MB model + second build tag + vendored `.so` was "at odds with the fast, lightweight, single container identity." That establishes the **constraint envelope** every future direction must respect:

1. **Browser-first.** Target users are non-technical (social media managers, bloggers, small-biz owners). The web UI *is* the product; they will not touch a CLI.
2. **Lightweight / single container / fast.** No giant ML models, no heavy runtime deps.
3. **In-memory, privacy-first, no disk, no accounts, no database.** This is the self-hoster selling point.
4. **Not an API, not a CLI.**

### Positioning insight (the moat)

The project's unfair advantage is **not compression** (Squoosh already nails that) and **not flexibility** (CLI tools own that). It is the **recipe + batch + packaging triad** — really a *"goal → ready-to-use deliverable" compiler*. Someone who has never heard of `apple-touch-icon` drags one logo and gets back a complete, correctly-named favicon pack with paste-in HTML. Nothing in the consumer-web space does that well.

So the strongest directions **deepen the packaging/recipe layer** or **raise output quality invisibly**; the weakest chase raw image manipulation that Squoosh/ImageMagick already own. The existing `Kind` plumbing (`KindImage` / `KindFaviconPack` / `KindDocumentPDF`, with `Result.Data` for single images vs `Result.Files` for packs) exists *specifically* to ship "everything you need for X" bundles — it is the rail the best new work runs on.

**A note on compression.** Plain "shrink this image" is *not* the moat — but that's a statement about Squoosh's *framing*, not the use case. Squoosh's goal is compression-as-a-tool: pick a codec, drag a slider, eyeball one image at a time. Our user's actual job is "I want these smaller for the web, give me the good version without thinking about it" — and they're already here doing the favicon / social / srcset thing. The win is not compressing *better* than Squoosh; it's making "just make these smaller" **one more recipe in the same drag-pick-download grid**, so this becomes the one tab they bookmark for everything. That reframes compression from a slider (off-strategy) into a few presets (dead-on the moat). See Direction 1.

### Chosen focus

Four directions are detailed here (the OG-card / text-overlay generator was considered and **deprioritized** — it is the heaviest, drifts toward a design tool, and needs bundled fonts + new render path). In recommended build order:

1. **Compress recipes** — 3 quality tiers, keep source format; ships today as pure presets, no new code paths.
2. **Responsive `srcset` pack** — new pack kind; highest differentiation-per-effort.
3. **Quality wins** — sharpen-after-downscale (+ optional target-filesize encoding); invisible, improves *every* preset.
4. **Preview + crop nudge** — fixes the biggest real quality gap for the core persona.

A near-free **share-via-URL + custom ZIP name** item is noted at the end as a stocking-stuffer.

---

## Direction 1 — Compress recipes (do first)

**What:** 3 "just make this smaller for the web" recipes that keep the source format and original dimensions — only the encoding is tuned. They appear as a **Compress** category in the same preset grid, selected and downloaded exactly like every other preset.

| Recipe | Behavior |
|---|---|
| **Compress · Best quality** | Near-visually-lossless. For "smaller but I won't notice." |
| **Compress · Balanced** | The everyday web default — strong shrink, great quality. |
| **Compress · Max savings** | Aggressive. For email, slow connections, tight limits. |

Output **keeps the source format** (JPEG→JPEG, PNG→PNG, WebP→WebP) so the user never gets a surprise file type. (A later "Compress · under [size]" recipe — a byte-budget target — belongs to Direction 3's target-filesize work and ships with it.)

**Why it fits:** This is compression reframed from *slider* to *recipe*. Squoosh's framing is compression-as-a-tool (pick codec, drag slider, one image at a time); our user's job is "give me the good smaller version without thinking." Because they're already here for the favicon / social / srcset jobs, "just make these smaller" becomes one more pill in the same drag-pick-download flow — the reason this app is the bookmark instead of Squoosh. Same UX, zero new surface.

**Weight:** Minimal — these are pure `KindImage` presets at original dimensions, identical in shape to the existing `website_webp` / `*_original` entries. No new code paths; ships on the current pipeline today.

**How it extends the architecture:**
- `processor/preset.go` — add 3 registry entries (keep-original-dimensions, source format mapped per input, tuned `Quality` / `Compression` / `Effort` per tier). Since output must mirror the *input* format, this is the one place that differs from a fixed-format preset: either add three-per-format entries, or let the preset carry a "match source" format sentinel resolved in `processImage`. Prefer the sentinel — fewer registry rows, one resolution point.
- `processor/pipeline.go` — if using the match-source sentinel, resolve it to the detected input format (via the existing `vips.DetermineImageType`) before `export()`. No other change.
- Frontend `frontend/src/lib/presets.js` + `PresetSelector.svelte` — add a **Compress** category with the three mirrored entries.

**Biggest risk:** Tier tuning needs to feel meaningfully different across formats (PNG has no quality knob — "max savings" PNG likely means palette/lossy-PNG, which is a small extra libvips step; for the first cut, PNG tiers can map to compression level only and a note that PNG savings are modest). Keep the three tiers honest so "max savings" is visibly smaller than "best quality."

---

## Direction 2 — Responsive `srcset` pack

**What:** One source image → a folder of widths (e.g. 320 / 640 / 960 / 1280 / 1920) each emitted in AVIF + WebP + JPEG fallback, plus a paste-in `<picture>` / `<img srcset sizes>` HTML snippet and a short README. The single most common real web task non-technical site owners get wrong, delivered drop-in.

**Why it fits:** It is almost a direct clone of the favicon-pack machinery, applied in the moat's own shape. Serves the exact "I don't know the recipe" need that defines the product. Nothing in the consumer-web space hands a non-technical user a ready `<picture>` bundle.

**Weight:** Light. Reuses `NewThumbnailWithSizeFromBuffer` per width and the existing `export()` per format; the snippet is a static generator exactly like the favicon HTML.

**How it extends the architecture:**
- `processor/preset.go` — add a new `Kind` (`KindSrcsetPack`) after `KindDocumentPDF` in the `const` block. Since the pack defines its own width/format matrix, the registry entry carries the master quality knobs but the size table lives next to the assembler (mirror of how `favicon` ignores `Width`/`Height` and `faviconSizes` lives in `favicon_vips.go`). Add one or a few presets (e.g. `srcset_web`).
- New `processor/srcset_vips.go` (`//go:build vips`) — `buildSrcsetPack(buf, p) Result` modelled on `buildFaviconPack`: loop widths × formats, fill `Result.Files` (`[]OutputFile`), leave `Data` nil.
- New `processor/srcset_assets.go` (tag-free) — the `<picture>` snippet + README generator, mirror of the favicon HTML asset file so it stays testable without libvips.
- `processor/pipeline.go` — in `processImage`, add a branch alongside the existing `if p.Kind == KindFaviconPack { return buildFaviconPack(...) }` to dispatch `KindSrcsetPack`.
- Handlers ZIP-writing already namespaces pack `Files` under a `<preset>/` folder — no change needed, since this is per-file (not a bundle), it slots into the existing pack path.
- Frontend `frontend/src/lib/presets.js` + `PresetSelector.svelte` — add the mirrored entry with label/category/help/accent.

**Biggest risk:** Combinatorial output (5 widths × 3 formats = 15 renders per source) can strain the flat-memory worker model on multi-file uploads. Mitigation: keep the default width/format set modest; this is a worker-accounting concern, not an envelope violation.

---

## Direction 3 — Quality wins: sharpen-after-downscale (+ optional target-filesize)

**What:** Apply a light unsharp-mask after downscaling (every serious pipeline — sharp, imgproxy — does this; this one currently skips it). Optionally, add a "hit a size budget" encoding mode that binary-searches quality to land under a target (e.g. "get me under 200 KB" for upload limits).

**Why it fits:** The sharpen change is nearly free and raises the floor on *every existing resize preset* — all social, thumbnail, banner, and the new srcset widths get visibly better output. It strengthens the "the output is just better than rolling your own" pitch under everything else. Both are pure libvips primitives — no new deps.

**Weight:** Sharpen = light (one conditional in `pipeline.go` + a `Sharpen bool` field). Target-filesize = medium (retry loop + SSE sub-progress).

**How it extends the architecture:**
- `processor/preset.go` — add `Sharpen bool` (and, for the optional part, `TargetBytes int`) to the `Preset` struct. Default the resize presets to `Sharpen: true`.
- `processor/pipeline.go` — after the `p.Resizes()` thumbnail branch and before `export()`, call `img.Sharpen(...)` with conservative defaults when `p.Sharpen`. For target-filesize, wrap `export()` in an encode-measure-retry loop (cap iterations hard, e.g. ≤5).
- `handlers/store.go` — only if target-filesize ships: the progress unit is currently one `(file, preset)` pair; iterative encoding has sub-progress, so either keep it coarse (one event on completion, as today) or extend the `progressEvent` schema. **Prefer keeping it coarse first** to avoid touching the wire schema.

**Biggest risk:** Over-sharpening soft sources (needs conservative defaults; the crop-preview from Direction 4 makes this trustworthy). For target-filesize, the retry loop's CPU/memory cost under the flat-memory model — cap iterations.

---

## Direction 4 — Preview + crop nudge

**What:** Before processing, show the user the crop box per image for fixed-aspect presets and let them drag to reposition. Today crop is hardcoded `vips.InterestingCentre`, so a portrait photo cropped to an Instagram square (or a wide logo to a favicon) frequently cuts off the subject.

**Why it fits:** Fixes the single biggest quality gap for the core persona (social media managers). Closes the loop on Direction 3 — a preview makes sharpening and crops trustworthy instead of blind.

**Weight:** Medium — mostly frontend, plus a per-file crop parameter threaded through the upload.

**How it extends the architecture:**
- Frontend — a crop-adjust step (drag the focal point / crop box) between preset selection and "Optimize." Optional and progressive: the default stays the dead-simple "drag, pick, download" flow; the nudge is opt-in so it doesn't break the core UX.
- `handlers/upload.go` — accept an optional per-file crop offset / focal point in the multipart form.
- `processor/preset.go` / `pipeline.go` — thread a focal point into the resize path. libvips supports attention-based crop (`vips.InterestingAttention`) and explicit-area crop; the simplest first cut is to swap `InterestingCentre` for `InterestingAttention` as a smarter default, then add explicit focal offset as the manual override. The SSE/job model is unaffected.

**Biggest risk:** Making it mandatory would break the "drag, pick, download" simplicity that is the moat. Must stay optional/progressive. A cheap intermediate win (no UI at all) is just switching the default to attention-based crop.

---

## Stocking-stuffer — Share-via-URL + custom ZIP name

Near-free, can ride alongside any of the above:
- Encode the selected preset set into the URL hash so a user can bookmark "my standard social export" or share it with a teammate (frontend-only; zero persistence — deliberately gets ~80% of "save my setup" with no database, staying inside the envelope).
- Let the user name the output ZIP (currently hardcoded in the download handler) — a one-line backend change plus a form field.

---

## The 'avoid' list (would violate the envelope)

1. **Any big model** — background-removal redux, AI upscaling, generative anything. Already ripped out once for exactly this reason. (libvips' *built-in* attention/entropy crop is fine — it's a primitive, not a model.)
2. **Persistence / accounts / job history** — needs a database; breaks in-memory + privacy + no-accounts. The share-via-URL approach is the deliberate substitute.
3. **A public API / webhook / folder-watch** — explicitly ruled out; owned by sharp/squoosh/imagemagick; pulls architecture toward stateful/multi-tenant.
4. **A full freeform editor** (layers, filters, brushes) — drags from "goal→deliverable compiler" into Canva/Photopea territory; unbounded scope; abandons the recipe-driven simplicity that is the moat.

---

## Critical files (for whoever executes any of these)

- `processor/preset.go` — `Kind` enum + `Preset` struct + registry; new pack kinds, `Sharpen`/`TargetBytes`/focal fields, and new presets all start here. **Tag-free — must stay govips-free.**
- `processor/pipeline.go` (`//go:build vips`) — the per-image render/export path; sharpen, target-filesize loop, crop focal point, and the new-pack dispatch all hook in here.
- `processor/favicon_vips.go` + its assets file — the template to copy for the srcset pack assembler and its static `<picture>` snippet.
- `handlers/store.go` — the `progressEvent` SSE schema and `runJob` accounting; only target-filesize sub-progress would touch this (prefer not to).
- `handlers/upload.go` — where a per-file crop/focal parameter enters.
- `frontend/src/lib/presets.js` + `frontend/src/components/PresetSelector.svelte` — mirrored registry + UI grid; every new preset/pack and the crop-preview/share-URL surface here.

## Verification (per direction, when built)

Real image processing requires the `vips` tag + libvips, i.e. the production Docker image:
- Tag-free unit tests run locally: `go test ./...` (covers `preset.go`, `ico.go`, handlers, and any new tag-free asset generator like `srcset_assets.go`).
- Vips-tagged tests: `go test -tags vips ./...` (covers the new `*_vips.go` assemblers).
- End-to-end: `docker build -t image-optimizer . && docker run -p 3000:3000 image-optimizer`, then upload through the UI and inspect the downloaded ZIP — confirm the srcset folder structure + snippet, eyeball the sharpening before/after, and check crop framing.
