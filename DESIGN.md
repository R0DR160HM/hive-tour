---
name: The Hive Tour
description: A language's home page and its tour, set as a book on cool paper, with each slide's text beside a live editor.
colors:
  paper: "#f6f7f8"
  ink: "#15171a"
  soft-ink: "#3d4249"
  muted-ink: "#5b6068"
  hairline: "#d5d8dd"
  wash: "#e9ebee"
  code-panel: "#eceef1"
  amber: "#e29b19"
  amber-ink: "#16120a"
  amber-text: "#8a5300"
  syntax-string: "#2f6a2a"
  syntax-literal: "#8b2f6b"
  syntax-comment: "#5a5f66"
  chapter-1: "#e7dbc1"
  chapter-1-read: "#ceb787"
  chapter-2: "#ead2c8"
  chapter-2-read: "#d6a997"
  chapter-3: "#e9d1d9"
  chapter-3-read: "#d0a3b2"
  chapter-4: "#dfd6ec"
  chapter-4-read: "#bdaed7"
  chapter-5: "#d1d8eb"
  chapter-5-read: "#a3b0d6"
  chapter-6: "#ccdfe7"
  chapter-6-read: "#95bbcb"
  chapter-7: "#cce1d7"
  chapter-7-read: "#93beaa"
  chapter-8: "#d8e0c5"
  chapter-8-read: "#aebe89"
  chapter-9: "#dcdad5"
  chapter-9-read: "#b9b6ae"
typography:
  display:
    fontFamily: "\"Hive Serif\" (Source Serif 4), Charter, \"Bitstream Charter\", \"Sitka Text\", Cambria, Georgia, serif"
    fontSize: "clamp(56px, 7vw, 88px)"
    fontWeight: 600
    lineHeight: 0.9
    letterSpacing: "-0.03em"
  headline:
    fontFamily: "\"Hive Serif\" (Source Serif 4), Charter, Georgia, serif"
    fontSize: "clamp(40px, 5vw, 64px)"
    fontWeight: 600
    lineHeight: 1.04
    letterSpacing: "-0.022em"
  title:
    fontFamily: "\"Hive Serif\" (Source Serif 4), Charter, Georgia, serif"
    fontSize: "clamp(28px, 3vw, 38px)"
    fontWeight: 600
    lineHeight: 1.12
    letterSpacing: "-0.016em"
  lede:
    fontFamily: "\"Hive Serif\" (Source Serif 4), Charter, Georgia, serif"
    fontSize: "24px"
    fontWeight: 500
    lineHeight: 1.36
    letterSpacing: "-0.01em"
  subhead:
    fontFamily: "\"Hive Serif\" (Source Serif 4), Charter, Georgia, serif"
    fontSize: "19px"
    fontWeight: 600
    lineHeight: 1.35
  body:
    fontFamily: "\"Hive Serif\" (Source Serif 4), Charter, Georgia, serif"
    fontSize: "18px"
    fontWeight: 400
    lineHeight: 1.62
  caption:
    fontFamily: "\"Hive Serif\" (Source Serif 4) italic, Georgia, serif"
    fontSize: "16px"
    fontWeight: 400
    lineHeight: 1.4
  label:
    fontFamily: "\"Hive Serif\" (Source Serif 4), Charter, Georgia, serif"
    fontSize: "14.5px"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "0.04em"
  code:
    fontFamily: "\"Hive Mono\" (Source Code Pro), ui-monospace, \"Cascadia Mono\", Menlo, Consolas, monospace"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.62
    letterSpacing: "0"
    fontFeature: "font-variant-ligatures: none"
  tile-symbol:
    fontFamily: "\"Hive Mono\" (Source Code Pro), ui-monospace, monospace"
    fontSize: "17px"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "-0.02em"
rounded:
  none: "0"
  pill: "999px"
  round: "50%"
spacing:
  gap-xs: "8px"
  gap-sm: "14px"
  gap-md: "28px"
  gap-lg: "56px"
  section: "128px"
  section-phone: "88px"
  gutter: "40px"
  gutter-phone: "20px"
  measure: "1280px"
components:
  button-primary:
    backgroundColor: "{colors.amber}"
    textColor: "{colors.amber-ink}"
    rounded: "{rounded.pill}"
    padding: "0 28px"
    height: "50px"
    typography: "{typography.subhead}"
  button-primary-hover:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.amber}"
  pill-outline:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    padding: "6px 20px"
    height: "46px"
  pill-outline-hover:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
  chart-tile:
    backgroundColor: "{colors.chapter-1}"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    width: "82px"
    height: "108px"
  chart-tile-read:
    backgroundColor: "{colors.chapter-1-read}"
    textColor: "{colors.ink}"
  slide-tile:
    backgroundColor: "{colors.chapter-2}"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    size: "136px"
    padding: "12px 14px"
  reads-tile:
    backgroundColor: "{colors.chapter-5}"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    padding: "7px 12px"
  code-panel:
    backgroundColor: "{colors.code-panel}"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    padding: "20px 24px"
    typography: "{typography.code}"
---

# Design System: The Hive Tour

## Overview

**Creative North Star: "The Language Set Out as a Book, with the Code Beside It"**

The site is a language's home page done properly and a tour that teaches the whole language, and it is set the way a good technical book is set: a cool white paper, one ink, a book serif for everything that is read and its sibling monospace for everything that is run. A slide is set the way a language's own tour is: the explanation on the left, read down the page, and its programs on the right in a pane held in view under a fixed running head, each program a panel of its own, with one console docked under them. The grammar is drawn as railroad diagrams (in the tour only). Every slide carries a tile in its chapter's colour beside its title, so a chapter has a colour and 85 slides stop looking like one; the contents are a plain chaptered list.

Density is a book's: an 18px body at a 66ch measure, generous paper between sections (128px on the front page), two columns where the explanation sits beside the programs it explains. Colour is rationed hard. Chapter colours only ever fill a tile; amber is the way on and where the reader is, and nothing else. Nothing moves: no entrance animation, no positional motion, only a 160ms colour change on the button, pills and tiles. The site ships no JavaScript beyond the playground's one script, so every state the design has (the reader's scheme, read slides, the current station) is drawn by CSS from what HTML and the server already know; only running an example needs the script.

Confirmed rejections: the SaaS landing page (gradient hero, feature-card rows, glow), a literal bee-and-honey theme, cream paper, and railroad drawing outside the tour.

**Key Characteristics:**
- Cool paper (never cream), one ink, flat surfaces separated by paper rather than lines.
- Source Serif 4 for headings and prose, Source Code Pro for programs, tiles and tokens; both embedded, nothing fetched.
- Nine muted, low-saturation chapter colours, each with a deeper read shade, filling tiles only.
- Amber on exactly two things: the one primary button and the reader's current station.
- Square tiles and panels; pills only for links that act as buttons; circles only for stations and drawn markers.
- Light and dark follow the system, with a no-JS cookie switch in the running head.

## Colors

A cool neutral book palette with one warm voice and nine quiet chapter fills.

### Primary
- **Hive Amber** (`amber`): the brand amber exactly as the mark paints it. A fill only, always with **Amber Ink** on it: the "Start the tour" pill, the current station dot on the track and on the pager line. Also the 2px underline under the active language and the hover underline colour of plain links. On paper it is under 3:1, so it is never text and never a focus ring.
- **Deep Amber** (`amber-text`): the same hue taken deep enough to be text (4.5:1+), reserved for the rare place amber must be read.

### Secondary
- **Chapter Fills** (`chapter-1` to `chapter-9`): nine muted hues in chapter order (ochre, apricot, rose, lilac, periwinkle, sky, mint, sage, stone). They fill chart tiles, the slide tile, the "In the tour" reads tiles and the chart key swatches.
- **Read Shades** (`chapter-1-read` to `chapter-9-read`): the deeper shade of each chapter a tile turns once its slide has been visited (`:visited`). All eighteen hold ink above 4.5:1 in both schemes.

### Neutral
- **Cool Paper** (`paper`): the page. Also the ground of open stations on the track.
- **Ink** (`ink`): text, rules, railroad track, pill outlines, focus ring, hover fill of pills and button.
- **Soft Ink** (`soft-ink`): secondary text (intro paragraph, code captions, tile chapter names, punctuation in code).
- **Muted Ink** (`muted-ink`): captions, EBNF lines, spec links, running-head links at rest; held above 4.5:1.
- **Hairline** (`hairline`): table row rules and the resting underline of plain links.
- **Wash** (`wash`) and **Code Panel** (`code-panel`): a shade off the paper, for inline code, program panels and the visited pager pill.
- **Syntax inks** (`syntax-string`, `syntax-literal`, `syntax-comment`): two quiet inks for strings and literal values; comments in a grey serif italic.

The dark scheme (`#121315` paper, `#e9e7e1` ink, darkened chapter fills `#373126`..`#32302d` with read shades `#544832`..`#4a4844`, amber `#e5a53a`) is written once and applied both under `prefers-color-scheme: dark` (unless the reader chose light) and under `data-theme=dark`. Values are in the sidecar.

### Named Rules
**The Tile-Only Rule.** Chapter colours fill tiles and nothing else: never text, borders, backgrounds of sections, or underlines. If it is not a tile (or a key swatch naming a tile's colour), it is paper or ink.

**The Two Ambers Rule.** Amber marks the way on (the single primary button) and where the reader is (the current station). A third amber element dilutes both. Hover underlines in amber are the only permitted trace beyond these.

**The Read Shade Rule.** Read state is `:visited` turning a tile to its chapter's deeper shade. It changes colour only, needs no script, and is the whole progress system on the chart.

## Typography

**Display Font:** Source Serif 4, embedded as "Hive Serif" (roman variable wght 400–700, opsz 8–60; italic static wght 400–600), with Charter / Cambria / Georgia fallbacks
**Body Font:** the same Source Serif 4
**Label/Mono Font:** Source Code Pro, embedded as "Hive Mono" (wght 400–700), with ui-monospace / Cascadia Mono / Menlo / Consolas fallbacks

**Character:** A book pairing from one family: an optically sized serif that is read, beside the monospace drawn to sit with it, for what is run. Italic is the captioning voice, as a book captions a figure.

### Hierarchy
- **Display** (600, clamp 56–88px, 0.9, -0.03em): the language name beside the mark on the front page; the 404 numeral at clamp 88–160px.
- **Headline** (600, clamp 40–64px, 1.04, -0.022em, balanced): slide titles and page titles.
- **Title** (600, clamp 28–38px, 1.12, -0.016em, balanced): front-page section headings.
- **Lede** (500, 24px, 1.36, 32ch max): the one sentence that says what Hive is (21px on phones).
- **Subhead** (600, 19px, 1.35): reasons and list headings; 18px italic 600 for rule names and column labels ("A good fit for", "Think twice if").
- **Body** (400, 18px, 1.62, 66ch max; 17px on phones): slide prose and front-page paragraphs; 17px/1.55 in dense lists.
- **Caption** (italic 400, 16px, 1.4): code captions, "In the tour" labels, the lead line over tables.
- **Label** (600, 14.5px, 1.2, +0.04em): running-head language switch and theme switch.
- **Code** (Mono 400, 14px / 13.5px on slides, 1.62, ligatures off): program panels and inline code at 0.84em on a wash.
- **Tile** (Mono 500 11px numbers; Mono 600 17px symbols on the chart, 32px on the slide tile; Serif 500 11px titles): the chart's own scale.

### Named Rules
**The Serif-Reads, Mono-Runs Rule.** Anything a reader could type into a `.hive` file (programs, tokens, commands, tile symbols, terminals in diagrams) is set in the monospace; everything else is the serif. Rule names in diagrams are serif italic, because they are names, not text to type.

**The Quiet Colour Rule.** Syntax highlighting uses a few quiet inks, each held at 4.5:1 or better on its panel in both schemes: keywords in a deep amber-brown (semibold) and builtins in the same ink, types in blue, strings in green, literals in plum, comments in grey (serif italic in static samples, monospace in editors so the caret lines up). Names and calls stay in ink. The editor repaints with the same rules as the server's lexer, from the same keyword and builtin lists.

## Layout

One centred column of 1280px max (`measure`) with 40px gutters (20px under 640px), with the running head aligned to the same column. Pages end with 112px of paper.

- **Front page:** the hero is a 5:7 split, text left (name, lede, intro at 58ch, then the ways in on two lines) and a whole program right, starting level with the name. Sections follow at 128px intervals (88px on phones), separated by paper alone; reasoned sections repeat the 5:7 split of argument and program/table. The page ends on the chart.
- **Slide:** two panes, 46:54. Left, aligned with the running head's column: chapter and "Slide n of 85" over a 2px progress line, the slide tile (112px) beside the headline (clamp 34–50px), the grammar fold, the prose, and the pager at the foot. Right, a wash-coloured pane held in view under the fixed running head (sticky, full remaining height): the programs scroll inside it, each a paper panel with a hairline border and its caption as the panel's header, and the console is docked under them (30% of the pane, at least 150px). Under 980px the panes stack: text, then programs and console, then the pager; the running head is fixed only above 980px.
- **Contents:** headline, one line of explanation (60ch), then the chaptered list.
- **Blog:** headline and one line, as on the contents page, then the posts down one 66ch column 52px apart, separated by paper: each a date in the caption's italic, its title as a heading-link (serif 600, clamp 24–30px) underlined in hairline until hover turns it amber, and its abstract in soft ink.
- **Post:** one column of 760px max: date, headline, then the prose at 66ch with its opening paragraph (the abstract) at 21px soft ink; section headings at the blog's title size, programs in the tour's code panel at the column's full width, links underlined in muted ink. It ends 64px down on the amber "Start the tour" pill and a plain "All posts" link.
- **Contents list:** chapters in CSS columns (3 × 260px min, 56px apart), each a heading (serif 600 19px) with a 12px square swatch of its colour, then its slides numbered (mono 13px muted numbers, serif 16.5px titles). A read slide is `:visited`, in muted ink. The same list ends the front page.

Breakpoints: **980px** all two-column rows and the slide's panes stack; **640px** phone (17px body, 96px slide tile, pager pills stack full-width, table rows stack, theme switch reduced to its drawn circle, running-head links never broken); **480px** the running head takes two lines, Contents and Blog under the mark.

**The Inline-Direction Rule.** `hive.ui` writes a row's direction as an inline `style`, so responsive overrides of flex direction, alignment and gap use `!important`, inside the responsive block only and for that purpose only. Nowhere else does the stylesheet use `!important` (besides the reduced-motion reset).

## Elevation & Depth

Flat. There are no drop shadows anywhere; depth is conveyed by tone (the code panel and wash a shade off the paper) and by ink lines. The only shadow syntax in the system is used as a stroke: an inset 2px ink ring on tile hover, a 2px ink ring round the amber station dot, and a 1px inset hairline on key swatches. Scroll shades are the one depth cue: code panels and diagram scrollers carry `local`/`scroll` background gradients so a faint ink shadow appears at an edge only when there is more content beyond it.

### Shadow Vocabulary
- **Tile focus ring** (`box-shadow: inset 0 0 0 2px var(--ink)`): hover and keyboard focus on chart and reads tiles, replacing the outline.
- **Station ring** (`box-shadow: 0 0 0 2px var(--ink)`): the amber current-station dot on the pager line.
- **Scroll shade** (12px gradient of ink at 16–18% at the scrolled edge): code panels and railroad scrollers only.

**The Flat Paper Rule.** Nothing floats. A surface is paper, a shade off paper, or a chapter fill; edges are ink strokes or nothing.

## Shapes

Square by default: tiles, code panels, inline code and figures have no radius. The pill (999px) is reserved for links that act as buttons: the amber primary button and the ink-outline pills (slide download link, previous/next). Circles are drawn markers only: track and pager stations, the half-filled theme-switch circle, the filled/hollow dots heading fit and caveat items. All strokes share one weight, 2px (`--rr`): pill outlines, the pager line, chevrons, the railroad track, the focus ring, the dashed placeholder tile. Chevrons are drawn as two 2px borders rotated, never an arrow glyph.

Railroad geometry (drawn by `hive.ui`'s canvas, one unit = one pixel, never scaled): 15px text, 9px mono advance, 16px half-height, capsules (rounded ends) for terminals and square boxes for rule names, 18px gaps, 10px bend radius, 2px track. Diagrams scroll sideways rather than shrink.

## Components

### Buttons
Confident and singular: there is one button on the site.
- **Shape:** fully rounded pill (999px), 50px tall.
- **Primary:** amber field, amber-ink text, serif 600 17px, 28px side padding. "Start the tour" on the front page and the 404.
- **Hover / Focus:** inks in (ink field; amber text on light, paper text on dark) over 160ms `cubic-bezier(.16,1,.3,1)`; a change of colour, never of position.
- **Outline pill:** paper field, 2px ink outline, ink text, serif 600 16px, 44–46px tall, 6px 20px padding; hover/focus fills ink with paper text. Used for a slide's download link, for previous/next, and (as a `<button>`, 22px side padding, no chevron) for a runnable example's Run, which reads Stop while it runs.
- **Plain links beside a button:** serif 600 16.5px underlined in hairline (2px), the underline turning amber on hover. A second, quieter line (500 15.5px, soft ink) carries repository and download on the front page.

### Chips (reads tiles)
- **Style:** small square chapter-filled links (7px 12px, serif 500 15px, ink), in a wrapping row under an italic "In the tour" caption; they name the slides that teach a front-page reason.
- **State:** visited turns the read shade; hover/focus draws the inset 2px ink ring.

### Cards / Containers
- **Code panel:** square, `code-panel` fill, 20px 24px padding, flush with the column, scrolls sideways on its own with scroll shades; captioned above in serif italic 16px soft ink.
- **Playground (runnable examples on slides):** under the code panel, 14px down, a wrapping row of the Run outline pill, "Put the original back" as a plain underlined serif 600 15.5px soft-ink button (shown only once the program is edited), and a state line in serif italic 15.5px muted ink ("You can edit this program and run it here.", "Compiling…", "It exited with status 2."). The panel itself is the editor: a transparent text box laid exactly over the painted program, repainted on every change, and the panel's border turns ink while editing. What comes back goes to the slide's one console, docked under the programs and shown as the page loads ("Run a program to see what it prints here."): a caption in the code-caption style naming what it is and which program ("What it printed · Variants, and what they bind") over mono 13.5px text that scrolls on its own. Starting one program stops any other that is running. No colour marks an error; the caption says what the text is. All of it is rendered `hidden` and uncovered by the script, so without it the slide is unchanged. The front page has none.
- **Tables:** rows separated by 1px hairlines top and bottom; first column is a mono or underlined-link label at a fixed width. These are the only hairlines in the system.

### Navigation
- **Running head:** mark (30px), "The Hive Tour", Contents and Blog in serif 600 16.5px; at the right, the scheme switch (a drawn half-filled circle plus "Dark"/"Light", only the scheme not on screen shown) and EN/PT in label type, muted at rest, ink on hover, the current language underlined 2px in amber.
- **Progress:** chapter name left and "Slide n of 85" right (serif 500 14.5px muted, tabular figures) over a 2px line in hairline, filled in ink as far as this slide.
- **Pager:** outline pills with drawn chevrons at the foot of the text pane; a visited pager pill rests on wash.

### Slide Tile
A square in the chapter's colour (112px; 84px on phones): number, symbol at 26px mono, chapter name in soft ink. It is the one place on a slide that carries chapter colour.

### Grammar Fold
One closed line under a slide's title: a 2px chevron that turns from pointing right to pointing down when open, "Grammar:" in serif 600 16px soft ink, and the rules' names after it in serif italic 500 16px muted. It is HTML's own `<details>`, so it opens without a script, and it holds every railroad figure the slide teaches. A slide opens on its explanation and its programs; the grammar is one click away.

### Railroad Figure
Rule name in serif italic 600 18px with a muted underlined spec link beside it, the diagram, the EBNF under it in mono 13.5px muted (76ch max), and an italic muted note when the rule is drawn from the compiler rather than the spec.

## Do's and Don'ts

### Do:
- **Do** keep amber to the one primary pill and the current station; everything else that needs emphasis uses ink, weight or an underline.
- **Do** fill tiles (slide tile, reads tiles, contents swatches) with chapter colours and let `:visited` turn the reads tiles to the read shade.
- **Do** set every program, token, command and tile symbol in Source Code Pro with ligatures off, and everything read in Source Serif 4.
- **Do** use the ink focus ring (2px solid ink, 3px offset), or the inset 2px ink ring on filled tiles.
- **Do** keep every interactive state in HTML and CSS: `:visited`, `:hover`, `:focus-visible`, server round-trips and the theme cookie. The playground's running states are the one exception, and even they are server-rendered markup the script only uncovers and relabels.
- **Do** draw marks (chevrons, stations, the theme circle, list dots) with CSS or the canvas at the 2px stroke weight.
- **Do** let code panels and diagrams scroll sideways with scroll shades instead of shrinking or widening the column.
- **Do** keep a slide's railroad figures inside its grammar fold, closed by default, so the slide opens on prose and programs.
- **Do** hold all text, and ink on every chapter fill and read shade, at 4.5:1 or better in both schemes.

### Don't:
- **Don't** use chapter colours for text, borders, section backgrounds or decoration.
- **Don't** add a second amber element, an amber focus ring or amber text on paper.
- **Don't** add drop shadows, gradients as decoration, glow, or feature-card rows.
- **Don't** use cream or warm paper; the paper is cool.
- **Don't** draw railroad diagrams outside the tour.
- **Don't** use arrow characters or icon glyphs for direction or status; draw them.
- **Don't** add hairlines between sections; only table rows get rules.
- **Don't** animate position or add entrance motion; colour changes at 160ms are the only motion.
- **Don't** use `!important` except to take back `hive.ui`'s inline row direction, alignment and gap at a breakpoint.
- **Don't** ship JavaScript beyond the playground's one script, external fonts, or any asset not embedded in the binary (the playground's compiled programs and Go's `wasm_exec.js` excepted).
