package processor

// Tag-free: these build the static text members of the favicon pack (manifest +
// HTML snippet) from constants, so they compile and unit-test locally without
// libvips.

// siteWebmanifest is the PWA manifest referencing the android-chrome PNGs in the
// pack. Kept minimal and theme-agnostic; users tweak name/colors to taste.
const siteWebmanifest = `{
  "name": "",
  "short_name": "",
  "icons": [
    {
      "src": "/android-chrome-192x192.png",
      "sizes": "192x192",
      "type": "image/png"
    },
    {
      "src": "/android-chrome-512x512.png",
      "sizes": "512x512",
      "type": "image/png"
    }
  ],
  "theme_color": "#ffffff",
  "background_color": "#ffffff",
  "display": "standalone"
}
`

// faviconReadme is the drop-in instruction file: the exact <head> tags to paste,
// plus where to put the files. Assumes the files are served from the site root;
// adjust the paths if you nest them in a subfolder.
const faviconReadme = `Favicon pack — drop-in instructions
====================================

1. Copy every file from this folder (except this README) into your site's
   web root, so they are served at e.g. https://example.com/favicon.ico

2. Paste these tags into the <head> of your HTML:

   <link rel="icon" href="/favicon.ico" sizes="any">
   <link rel="icon" type="image/png" sizes="16x16" href="/favicon-16x16.png">
   <link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png">
   <link rel="icon" type="image/png" sizes="48x48" href="/favicon-48x48.png">
   <link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">
   <link rel="manifest" href="/site.webmanifest">

3. (Optional) Open site.webmanifest and fill in "name" / "short_name" and the
   theme/background colors for the PWA install experience.

Files:
   favicon.ico                  classic multi-size icon (16/32/48)
   favicon-16x16.png            browser tab
   favicon-32x32.png            browser tab / taskbar
   favicon-48x48.png            high-DPI tab
   apple-touch-icon.png         iOS home screen (180x180)
   android-chrome-192x192.png   Android home screen
   android-chrome-512x512.png   Android splash / PWA
   site.webmanifest             PWA manifest
`

// faviconManifestBytes returns the manifest file contents.
func faviconManifestBytes() []byte { return []byte(siteWebmanifest) }

// faviconReadmeBytes returns the README/snippet file contents.
func faviconReadmeBytes() []byte { return []byte(faviconReadme) }
