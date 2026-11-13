# Inter fallback font

To add glyphs to the Inter fallback font:

1. Install [FontForge](https://fontforge.org/en-US/downloads/mac-dl/).
2. Download the [Inter](https://rsms.me/inter/) typefaces.
3. Open the Inter Regular typeface from the web directory in FontForge.
4. Copy the desired glyph using Edit > Copy.
5. Open [Inter-fallback.sfd](Inter-fallback.sfd) in FontForge.
6. Select the glyph's Unicode slot using Encoding > Goto, then paste the glyph.
7. Generate a `.ttf` file.
8. Install [fonttools](https://fonttools.readthedocs.io/en/latest/subset/index.html),
   for example with `brew install fonttools` on macOS.
9. From `apps/explorer`, adjust the Unicode range and run:

   ```sh
   pyftsubset <path-to-ttf-file> \
     --output-file=./public/static/fonts/Inter-fallback.woff2 \
     --flavor=woff2 \
     --unicodes=U+2192
   ```
